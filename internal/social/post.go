package social

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Post struct {
	Version     int            `json:"version"`
	ID          string         `json:"id"`
	Author      PublicIdentity `json:"author"`
	AuthorNode  string         `json:"author_node"`
	CreatedUnix int64          `json:"created_unix"`
	Text        string         `json:"text"`
	Tags        []string       `json:"tags,omitempty"`
	Signature   string         `json:"signature"`
}

func NewPost(identity *Identity, profile PublicIdentity, authorNode, text string, tags []string, now time.Time) (Post, error) {
	priv, err := identity.private()
	if err != nil {
		return Post{}, err
	}
	if _, err := profile.Verify(); err != nil || profile.ID != identity.ID {
		return Post{}, errors.New("author profile does not match identity")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return Post{}, errors.New("post is empty")
	}
	if len([]byte(text)) > 256<<10 {
		return Post{}, errors.New("post is too large")
	}
	p := Post{Version: 1, Author: profile, AuthorNode: strings.TrimSpace(authorNode), CreatedUnix: now.Unix(), Text: text, Tags: normalizeTags(tags)}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return Post{}, err
	}
	h := sha256.Sum256(append(postToSign(p), nonce...))
	p.ID = base64.RawURLEncoding.EncodeToString(h[:])
	p.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(priv, postToSign(p)))
	return p, nil
}

func (p Post) Verify() error {
	if p.Version != 1 || p.ID == "" || strings.TrimSpace(p.Text) == "" {
		return errors.New("invalid post")
	}
	pub, err := p.Author.Verify()
	if err != nil {
		return err
	}
	sig, err := base64.RawStdEncoding.DecodeString(p.Signature)
	if err != nil || !ed25519.Verify(pub, postToSign(p), sig) {
		return errors.New("invalid post signature")
	}
	return nil
}

func postToSign(p Post) []byte {
	h := sha256.Sum256([]byte(p.Text))
	return []byte(fmt.Sprintf("knotroute/post/v1|%s|%s|%d|%x|%s", p.Author.ID, p.AuthorNode, p.CreatedUnix, h[:], strings.Join(normalizeTags(p.Tags), ",")))
}

func normalizeTags(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(raw, "#")))
		if v == "" || seen[v] || len(v) > 40 {
			continue
		}
		seen[v] = true
		out = append(out, v)
		if len(out) == 16 {
			break
		}
	}
	return out
}
