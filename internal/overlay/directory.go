package overlay

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/directory"
	"github.com/localzet/knotroute/internal/naming"
	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/protocol"
	"github.com/localzet/knotroute/internal/serviceid"
	"github.com/localzet/knotroute/internal/serviceidentity"
)

type publishedService struct {
	cfg           config.Service
	identity      *serviceidentity.Identity
	revision      atomic.Uint64
	revisionPath  string
	introMu       sync.Mutex
	introSessions map[nodeid.ID]*introSession
	introDialing  map[nodeid.ID]bool
}
type descriptorResult struct {
	descriptor directory.Descriptor
	found      bool
}

type directoryState struct {
	mu          sync.RWMutex
	descriptors map[serviceid.ID]directory.Descriptor
	pending     map[[16]byte]chan descriptorResult
	local       map[string]*publishedService
	wake        chan struct{}
}

func (n *Node) initDirectory() error {
	n.directory.descriptors = map[serviceid.ID]directory.Descriptor{}
	n.directory.pending = map[[16]byte]chan descriptorResult{}
	n.directory.local = map[string]*publishedService{}
	n.directory.wake = make(chan struct{}, 1)
	for _, svc := range n.cfg.Services {
		if !svc.Publish {
			continue
		}
		path := svc.IdentityFile
		if path == "" && n.cfg.Path != "" {
			path = filepath.Join(filepath.Dir(n.cfg.Path), "services", svc.Name+".identity.json")
		}
		var id *serviceidentity.Identity
		var err error
		if path == "" {
			id, err = serviceidentity.Generate()
		} else {
			id, err = serviceidentity.LoadOrCreate(path)
		}
		if err != nil {
			return fmt.Errorf("service %s identity: %w", svc.Name, err)
		}
		pub := &publishedService{cfg: svc, identity: id}
		if path != "" {
			pub.revisionPath = path + ".revision"
			revision, err := loadDescriptorRevision(pub.revisionPath)
			if err != nil {
				return fmt.Errorf("service %s descriptor revision: %w", svc.Name, err)
			}
			pub.revision.Store(revision)
		} else {
			pub.revision.Store(descriptorRevisionEpoch())
		}
		n.directory.local[svc.Name] = pub
	}
	return nil
}

func descriptorRevisionEpoch() uint64 {
	// Legacy builds used a process-local counter starting at zero. Seeding new
	// revision state from wall-clock milliseconds makes the first descriptor
	// after upgrading newer than any realistic legacy counter while the
	// persisted sidecar file keeps revisions monotonic across later restarts.
	return uint64(time.Now().UTC().UnixMilli()) << 16
}

func loadDescriptorRevision(path string) (uint64, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return descriptorRevisionEpoch(), nil
	}
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", path, err)
	}
	return value, nil
}

func saveDescriptorRevision(path string, revision uint64) error {
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil && dir != "." {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".descriptor-revision-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := fmt.Fprintf(tmp, "%d\n", revision); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func nextDescriptorRevision(s *publishedService) (uint64, error) {
	revision := s.revision.Add(1)
	if err := saveDescriptorRevision(s.revisionPath, revision); err != nil {
		return 0, fmt.Errorf("persist descriptor revision: %w", err)
	}
	return revision, nil
}

func (n *Node) serviceDomains() map[string]string {
	out := map[string]string{}
	n.directory.mu.RLock()
	defer n.directory.mu.RUnlock()
	for name, s := range n.directory.local {
		out[name] = naming.ServiceCanonicalDomain(s.identity.ID)
	}
	return out
}

func (n *Node) directoryLoop() {
	defer n.wg.Done()
	ticker := time.NewTicker(n.cfg.DescriptorPublishInterval())
	clean := time.NewTicker(time.Minute)
	defer ticker.Stop()
	defer clean.Stop()
	var wakeTimer *time.Timer
	var wakeC <-chan time.Time
	defer func() {
		if wakeTimer != nil {
			wakeTimer.Stop()
		}
	}()
	n.publishAllDescriptors()
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.publishAllDescriptors()
		case <-n.directory.wake:
			// Coalesce topology churn into one near-term publication. This is a
			// leading-edge debounce: frequent LSAs must not postpone publication
			// forever.
			if wakeC == nil {
				if wakeTimer == nil {
					wakeTimer = time.NewTimer(350 * time.Millisecond)
				} else {
					wakeTimer.Reset(350 * time.Millisecond)
				}
				wakeC = wakeTimer.C
			}
		case <-wakeC:
			wakeC = nil
			n.publishAllDescriptors()
		case <-clean.C:
			n.cleanDescriptors()
		}
	}
}

func (n *Node) wakeDirectoryPublisher() {
	if n.directory.wake == nil {
		return
	}
	select {
	case n.directory.wake <- struct{}{}:
	default:
	}
}

func (n *Node) publishAllDescriptors() {
	n.directory.mu.RLock()
	list := make([]*publishedService, 0, len(n.directory.local))
	for _, s := range n.directory.local {
		list = append(list, s)
	}
	n.directory.mu.RUnlock()
	for _, s := range list {
		n.ensureIntroductionPoints(s)
		if err := n.publishDescriptor(s); err != nil && n.ctx.Err() == nil {
			n.addEvent("warn", "publish "+s.cfg.Name+": "+err.Error())
		}
	}
}

func (n *Node) publishDescriptor(s *publishedService) error {
	intros := n.activeIntros(s)
	if len(intros) == 0 {
		return errors.New("no active introduction points")
	}
	revision, err := nextDescriptorRevision(s)
	if err != nil {
		return err
	}
	d, err := directory.New(s.identity, n.network, intros, revision, n.cfg.DescriptorTTL(), s.cfg.Metadata)
	if err != nil {
		return err
	}
	n.storeDescriptor(d)
	raw, _ := json.Marshal(d)
	replicas := n.closestDirectoryNodes(s.identity.ID, n.cfg.Directory.Replicas)
	for _, dst := range replicas {
		if dst == n.id.ID {
			continue
		}
		packet := n.newPacket(protocol.PacketDescriptorPut, dst, [16]byte{}, 0, raw)
		if err := n.sendPacketWithRetry(packet, 3*time.Second); err != nil {
			n.addEvent("warn", "descriptor replica "+dst.Short()+": "+err.Error())
		}
	}
	n.addEvent("info", "published service "+s.cfg.Name+" as "+naming.ServiceCanonicalDomain(s.identity.ID))
	return nil
}

func (n *Node) selectIntroductionPoints(count int) []nodeid.ID {
	if count <= 0 {
		count = 3
	}
	n.topologyMu.RLock()
	ids := make([]nodeid.ID, 0, len(n.routes))
	for id := range n.routes {
		ids = append(ids, id)
	}
	n.topologyMu.RUnlock()
	if len(ids) == 0 {
		return []nodeid.ID{n.id.ID}
	}
	sort.Slice(ids, func(i, j int) bool { return nodeid.Compare(ids[i], ids[j]) < 0 })
	if count > len(ids) {
		count = len(ids)
	}
	return append([]nodeid.ID(nil), ids[:count]...)
}

func (n *Node) storeDescriptor(d directory.Descriptor) bool {
	id, err := d.Verify(time.Now(), n.network)
	if err != nil {
		return false
	}
	n.directory.mu.Lock()
	defer n.directory.mu.Unlock()
	old, ok := n.directory.descriptors[id]
	if ok && old.Revision >= d.Revision {
		return false
	}
	n.directory.descriptors[id] = d
	return true
}
func (n *Node) cleanDescriptors() {
	now := time.Now().Unix()
	n.directory.mu.Lock()
	for id, d := range n.directory.descriptors {
		if d.ExpiresUnix <= now {
			delete(n.directory.descriptors, id)
		}
	}
	n.directory.mu.Unlock()
}

func (n *Node) closestDirectoryNodes(id serviceid.ID, count int) []nodeid.ID {
	ids := []nodeid.ID{n.id.ID}
	n.topologyMu.RLock()
	for nid := range n.routes {
		ids = append(ids, nid)
	}
	n.topologyMu.RUnlock()
	sort.Slice(ids, func(i, j int) bool { return xorLess(ids[i], ids[j], id) })
	if count > len(ids) {
		count = len(ids)
	}
	return ids[:count]
}
func xorLess(a, b nodeid.ID, target serviceid.ID) bool {
	for i := 0; i < 32; i++ {
		x := a[i] ^ target[i]
		y := b[i] ^ target[i]
		if x < y {
			return true
		}
		if x > y {
			return false
		}
	}
	return nodeid.Compare(a, b) < 0
}

func (n *Node) LookupService(ctx context.Context, id serviceid.ID) (directory.Descriptor, error) {
	return n.lookupService(ctx, id, true)
}

func (n *Node) lookupService(ctx context.Context, id serviceid.ID, useCache bool) (directory.Descriptor, error) {
	if useCache {
		n.directory.mu.RLock()
		if d, ok := n.directory.descriptors[id]; ok && d.ExpiresUnix > time.Now().Unix() {
			n.directory.mu.RUnlock()
			return d, nil
		}
		n.directory.mu.RUnlock()
	}
	var reqID [16]byte
	if _, err := rand.Read(reqID[:]); err != nil {
		return directory.Descriptor{}, err
	}
	ch := make(chan descriptorResult, 16)
	n.directory.mu.Lock()
	n.directory.pending[reqID] = ch
	n.directory.mu.Unlock()
	defer func() { n.directory.mu.Lock(); delete(n.directory.pending, reqID); n.directory.mu.Unlock() }()
	raw, _ := json.Marshal(protocol.DescriptorGet{ServiceID: id.String()})
	nodes := n.closestDirectoryNodes(id, n.cfg.Directory.Replicas)
	sent := 0
	for _, dst := range nodes {
		if dst == n.id.ID {
			continue
		}
		if n.sendPacket(n.newPacket(protocol.PacketDescriptorGet, dst, reqID, 0, raw)) == nil {
			sent++
		}
	}
	if sent == 0 {
		return directory.Descriptor{}, errors.New("no directory replicas reachable")
	}
	timeout := time.NewTimer(n.cfg.DescriptorLookupTimeout())
	defer timeout.Stop()
	var settle *time.Timer
	var settleC <-chan time.Time
	defer func() {
		if settle != nil {
			settle.Stop()
		}
	}()
	var best directory.Descriptor
	received := 0
	returnBest := func() (directory.Descriptor, error) {
		if best.ServiceID == "" {
			return directory.Descriptor{}, errors.New("service descriptor not found")
		}
		n.storeDescriptor(best)
		return best, nil
	}
	for {
		select {
		case <-ctx.Done():
			return directory.Descriptor{}, ctx.Err()
		case <-n.ctx.Done():
			return directory.Descriptor{}, errors.New("node stopped")
		case <-timeout.C:
			return returnBest()
		case <-settleC:
			return returnBest()
		case r := <-ch:
			received++
			if r.found {
				if _, err := r.descriptor.Verify(time.Now(), n.network); err == nil && (best.ServiceID == "" || r.descriptor.Revision > best.Revision) {
					best = r.descriptor
					if settle == nil {
						settle = time.NewTimer(250 * time.Millisecond)
						settleC = settle.C
					}
				}
			}
			if received >= sent {
				return returnBest()
			}
		}
	}
}

func (n *Node) invalidateServiceDescriptor(id serviceid.ID) {
	n.directory.mu.Lock()
	delete(n.directory.descriptors, id)
	n.directory.mu.Unlock()
}

func (n *Node) handleDirectoryPacket(packet protocol.Packet) bool {
	switch packet.Kind {
	case protocol.PacketDescriptorPut:
		var d directory.Descriptor
		if json.Unmarshal(packet.Payload, &d) == nil {
			n.storeDescriptor(d)
		}
		return true
	case protocol.PacketDescriptorGet:
		var q protocol.DescriptorGet
		if json.Unmarshal(packet.Payload, &q) != nil {
			return true
		}
		id, err := serviceid.Parse(q.ServiceID)
		if err != nil {
			return true
		}
		n.directory.mu.RLock()
		d, ok := n.directory.descriptors[id]
		n.directory.mu.RUnlock()
		resp := protocol.DescriptorResponse{Found: ok}
		if ok {
			raw, _ := json.Marshal(d)
			resp.Descriptor = raw
		}
		raw, _ := json.Marshal(resp)
		_ = n.sendPacket(n.newPacket(protocol.PacketDescriptorResponse, packet.Src, packet.StreamID, 0, raw))
		return true
	case protocol.PacketDescriptorResponse:
		var resp protocol.DescriptorResponse
		if json.Unmarshal(packet.Payload, &resp) != nil {
			return true
		}
		r := descriptorResult{found: resp.Found}
		if resp.Found && len(resp.Descriptor) > 0 {
			if json.Unmarshal(resp.Descriptor, &r.descriptor) != nil {
				return true
			}
		}
		n.directory.mu.RLock()
		ch := n.directory.pending[packet.StreamID]
		n.directory.mu.RUnlock()
		if ch != nil {
			select {
			case ch <- r:
			default:
			}
		}
		return true
	default:
		return false
	}
}
