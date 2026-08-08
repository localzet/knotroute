package clientruntime

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"github.com/localzet/knotroute/internal/naming"
	"github.com/localzet/knotroute/internal/social"
)

const socialServiceName = "kr-chat"
const socialFrameLimit = 1 << 20

type socialRequest struct {
	Version int             `json:"version"`
	Op      string          `json:"op"`
	Message *social.Message `json:"message,omitempty"`
}

type socialResponse struct {
	OK      bool                   `json:"ok"`
	Error   string                 `json:"error,omitempty"`
	Profile *social.PublicIdentity `json:"profile,omitempty"`
	Posts   []social.Post          `json:"posts,omitempty"`
}

func (r *Runtime) startSocialListener() (net.Listener, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.socialListener = listener
	r.mu.Unlock()
	return listener, nil
}

func (r *Runtime) serveSocial(ctx context.Context, listener net.Listener) {
	r.socialWG.Add(1)
	defer r.socialWG.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				return
			}
		}
		r.socialWG.Add(1)
		go func() {
			defer r.socialWG.Done()
			defer conn.Close()
			r.handleSocial(conn)
		}()
	}
}

func (r *Runtime) handleSocial(conn net.Conn) {
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	var req socialRequest
	if err := readSocialFrame(conn, &req); err != nil || req.Version != 1 {
		return
	}
	switch req.Op {
	case "profile":
		profile, err := r.UserProfile()
		if err != nil {
			_ = writeSocialFrame(conn, socialResponse{Error: err.Error()})
			return
		}
		_ = writeSocialFrame(conn, socialResponse{OK: true, Profile: &profile})
	case "message":
		if req.Message == nil {
			_ = writeSocialFrame(conn, socialResponse{Error: "missing message"})
			return
		}
		if err := req.Message.Verify(); err != nil {
			_ = writeSocialFrame(conn, socialResponse{Error: err.Error()})
			return
		}
		if req.Message.RecipientID != r.userIdentity.ID {
			_ = writeSocialFrame(conn, socialResponse{Error: "message recipient does not match this user"})
			return
		}
		contact := social.Contact{Profile: req.Message.Sender, Node: req.Message.SenderNode}
		_ = r.socialStore.PutContact(contact)
		if err := r.socialStore.PutMessage(req.Message.Sender.ID, *req.Message); err != nil {
			_ = writeSocialFrame(conn, socialResponse{Error: err.Error()})
			return
		}
		_ = writeSocialFrame(conn, socialResponse{OK: true})
	case "feed":
		state := r.socialStore.Snapshot()
		posts := make([]social.Post, 0, len(state.Posts))
		for _, post := range state.Posts {
			if post.Author.ID == r.userIdentity.ID {
				posts = append(posts, post)
			}
			if len(posts) == 64 {
				break
			}
		}
		_ = writeSocialFrame(conn, socialResponse{OK: true, Posts: posts})
	default:
		_ = writeSocialFrame(conn, socialResponse{Error: "unsupported social operation"})
	}
}

func (r *Runtime) UserProfile() (social.PublicIdentity, error) {
	state := r.socialStore.Snapshot()
	return r.userIdentity.Public(state.DisplayName, state.Bio, state.AvatarHash, time.Now().UTC())
}

func (r *Runtime) UserID() string            { return r.userIdentity.ID }
func (r *Runtime) SocialState() social.State { return r.socialStore.Snapshot() }

func (r *Runtime) SetUserProfile(displayName, bio string) error {
	if strings.TrimSpace(displayName) == "" {
		return errors.New("display name is required")
	}
	return r.socialStore.SetProfile(strings.TrimSpace(displayName), strings.TrimSpace(bio), r.socialStore.Snapshot().AvatarHash)
}

func (r *Runtime) AddContact(ctx context.Context, nodeReference, alias string) (social.Contact, error) {
	profile, nodeID, err := r.fetchRemoteProfile(ctx, nodeReference)
	if err != nil {
		return social.Contact{}, err
	}
	contact := social.Contact{Profile: profile, Node: nodeID, Alias: strings.TrimSpace(alias)}
	if err := r.socialStore.PutContact(contact); err != nil {
		return social.Contact{}, err
	}
	return contact, nil
}

func (r *Runtime) SendMessage(ctx context.Context, userID, body string) (social.Message, error) {
	state := r.socialStore.Snapshot()
	contact, ok := state.Contacts[userID]
	if !ok {
		return social.Message{}, errors.New("contact is not known")
	}
	r.mu.RLock()
	node := r.node
	localNode := ""
	if r.identity != nil {
		localNode = r.identity.ID.String()
	}
	r.mu.RUnlock()
	if node == nil {
		return social.Message{}, errors.New("KnotRoute node is not running")
	}
	remote, err := naming.ResolveNodeReference(contact.Node, r.Config().Aliases)
	if err != nil {
		return social.Message{}, err
	}
	profile, err := r.UserProfile()
	if err != nil {
		return social.Message{}, err
	}
	message, err := social.NewMessage(r.userIdentity, profile, localNode, userID, body, "", time.Now().UTC())
	if err != nil {
		return social.Message{}, err
	}
	conn, err := node.OpenCircuitStream(ctx, remote, socialServiceName)
	if err != nil {
		return social.Message{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	if err := writeSocialFrame(conn, socialRequest{Version: 1, Op: "message", Message: &message}); err != nil {
		return social.Message{}, err
	}
	var resp socialResponse
	if err := readSocialFrame(conn, &resp); err != nil {
		return social.Message{}, err
	}
	if !resp.OK {
		return social.Message{}, errors.New(resp.Error)
	}
	if err := r.socialStore.PutMessage(userID, message); err != nil {
		return social.Message{}, err
	}
	return message, nil
}

func (r *Runtime) CreatePost(text string, tags []string) (social.Post, error) {
	r.mu.RLock()
	localNode := ""
	if r.identity != nil {
		localNode = r.identity.ID.String()
	}
	r.mu.RUnlock()
	profile, err := r.UserProfile()
	if err != nil {
		return social.Post{}, err
	}
	post, err := social.NewPost(r.userIdentity, profile, localNode, text, tags, time.Now().UTC())
	if err != nil {
		return social.Post{}, err
	}
	if err := r.socialStore.PutPost(post); err != nil {
		return social.Post{}, err
	}
	return post, nil
}

func (r *Runtime) FetchContactFeed(ctx context.Context, userID string) ([]social.Post, error) {
	state := r.socialStore.Snapshot()
	contact, ok := state.Contacts[userID]
	if !ok {
		return nil, errors.New("contact is not known")
	}
	r.mu.RLock()
	node := r.node
	r.mu.RUnlock()
	if node == nil {
		return nil, errors.New("KnotRoute node is not running")
	}
	remote, err := naming.ResolveNodeReference(contact.Node, r.Config().Aliases)
	if err != nil {
		return nil, err
	}
	conn, err := node.OpenCircuitStream(ctx, remote, socialServiceName)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	if err := writeSocialFrame(conn, socialRequest{Version: 1, Op: "feed"}); err != nil {
		return nil, err
	}
	var resp socialResponse
	if err := readSocialFrame(conn, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, errors.New(resp.Error)
	}
	out := make([]social.Post, 0, len(resp.Posts))
	for _, post := range resp.Posts {
		if post.Verify() == nil && post.Author.ID == userID {
			_ = r.socialStore.PutPost(post)
			out = append(out, post)
		}
	}
	return out, nil
}

func (r *Runtime) fetchRemoteProfile(ctx context.Context, nodeReference string) (social.PublicIdentity, string, error) {
	r.mu.RLock()
	node := r.node
	r.mu.RUnlock()
	if node == nil {
		return social.PublicIdentity{}, "", errors.New("KnotRoute node is not running")
	}
	id, err := naming.ResolveNodeReference(strings.TrimSpace(nodeReference), r.Config().Aliases)
	if err != nil {
		return social.PublicIdentity{}, "", err
	}
	conn, err := node.OpenCircuitStream(ctx, id, socialServiceName)
	if err != nil {
		return social.PublicIdentity{}, "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	if err := writeSocialFrame(conn, socialRequest{Version: 1, Op: "profile"}); err != nil {
		return social.PublicIdentity{}, "", err
	}
	var resp socialResponse
	if err := readSocialFrame(conn, &resp); err != nil {
		return social.PublicIdentity{}, "", err
	}
	if !resp.OK || resp.Profile == nil {
		if resp.Error == "" {
			resp.Error = "remote profile unavailable"
		}
		return social.PublicIdentity{}, "", errors.New(resp.Error)
	}
	if _, err := resp.Profile.Verify(); err != nil {
		return social.PublicIdentity{}, "", err
	}
	return *resp.Profile, id.String(), nil
}

func writeSocialFrame(w io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(raw) > socialFrameLimit {
		return errors.New("social frame too large")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(raw)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err = w.Write(raw)
	return err
}
func readSocialFrame(r io.Reader, value any) error {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return err
	}
	size := int(binary.BigEndian.Uint32(header[:]))
	if size <= 0 || size > socialFrameLimit {
		return errors.New("invalid social frame size")
	}
	raw := make([]byte, size)
	if _, err := io.ReadFull(r, raw); err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}
