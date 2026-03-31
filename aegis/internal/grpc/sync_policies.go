package grpc

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	pb "github.com/jasonKoogler/abraxis/aegis/api/grpc/aegispb"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
)

// SyncPolicies streams the current OPA policy files followed by incremental
// policy updates. The initial snapshot reads all .rego files from the
// configured policy directory (or single file) and sends them as a single
// PolicyEvent. After the snapshot the stream stays open and forwards events
// from the policy event bus until the client disconnects.
func (s *AegisAuthServer) SyncPolicies(req *pb.SyncRequest, stream grpc.ServerStreamingServer[pb.PolicyEvent]) error {
	ctx := stream.Context()
	subID := uuid.New().String()

	// Subscribe BEFORE reading policies so we don't miss concurrent updates.
	events := s.policyEventBus.Subscribe(subID)
	defer s.policyEventBus.Unsubscribe(subID)

	// --- Snapshot phase ---
	policies, err := s.loadPolicies()
	if err != nil {
		s.logger.Error("SyncPolicies: failed to load policy files", log.Error(err))
		return fmt.Errorf("failed to load policy files: %w", err)
	}

	snapshot := &pb.PolicyEvent{
		Version:  s.policyEventBus.NextVersion(),
		Policies: policies,
	}
	if err := stream.Send(snapshot); err != nil {
		return err
	}

	// --- Incremental phase ---
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if err := stream.Send(event); err != nil {
				return err
			}
		}
	}
}

// loadPolicies reads .rego files from the configured policy path. If the path
// points to a single file it returns that file. If it points to a directory it
// walks the directory and collects all .rego files.
func (s *AegisAuthServer) loadPolicies() (map[string]string, error) {
	policies := make(map[string]string)

	info, err := os.Stat(s.policyFilePath)
	if err != nil {
		return nil, fmt.Errorf("policy path %s: %w", s.policyFilePath, err)
	}

	if !info.IsDir() {
		data, err := os.ReadFile(s.policyFilePath)
		if err != nil {
			return nil, fmt.Errorf("reading policy file %s: %w", s.policyFilePath, err)
		}
		policies[filepath.Base(s.policyFilePath)] = string(data)
		return policies, nil
	}

	entries, err := os.ReadDir(s.policyFilePath)
	if err != nil {
		return nil, fmt.Errorf("reading policy directory %s: %w", s.policyFilePath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".rego" {
			continue
		}
		fullPath := filepath.Join(s.policyFilePath, entry.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("reading policy file %s: %w", fullPath, err)
		}
		policies[entry.Name()] = string(data)
	}

	return policies, nil
}

// PublishPoliciesChanged reloads policies from disk and publishes the updated
// set to all connected SyncPolicies streams.
func (s *AegisAuthServer) PublishPoliciesChanged() error {
	policies, err := s.loadPolicies()
	if err != nil {
		return fmt.Errorf("reloading policies for publish: %w", err)
	}

	s.policyEventBus.Publish(&pb.PolicyEvent{
		Policies: policies,
	})
	return nil
}
