package store

import (
	"context"
	"testing"
)

func TestSetReaction_NewUpsert(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	if err := s.SetReaction(ctx, "w1", "alice", "👍", 100); err != nil {
		t.Fatalf("SetReaction: %v", err)
	}
	rs, err := s.ListReactions(ctx, "w1")
	if err != nil {
		t.Fatalf("ListReactions: %v", err)
	}
	if len(rs) != 1 || rs[0].Emoji != "👍" || rs[0].SenderJID != "alice" {
		t.Errorf("ListReactions = %+v, want [{w1 alice 👍 100}]", rs)
	}
}

func TestSetReaction_Replace(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	_ = s.SetReaction(ctx, "w1", "alice", "👍", 100)
	if err := s.SetReaction(ctx, "w1", "alice", "❤", 200); err != nil {
		t.Fatalf("SetReaction replace: %v", err)
	}
	rs, _ := s.ListReactions(ctx, "w1")
	if len(rs) != 1 || rs[0].Emoji != "❤" || rs[0].TS != 200 {
		t.Errorf("replace failed: %+v", rs)
	}
}

func TestSetReaction_EmptyRemoves(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	_ = s.SetReaction(ctx, "w1", "alice", "👍", 100)
	if err := s.SetReaction(ctx, "w1", "alice", "", 200); err != nil {
		t.Fatalf("SetReaction remove: %v", err)
	}
	rs, _ := s.ListReactions(ctx, "w1")
	if len(rs) != 0 {
		t.Errorf("empty emoji should remove; got %+v", rs)
	}
}

func TestSetReaction_DifferentSendersCoexist(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	_ = s.SetReaction(ctx, "w1", "alice", "👍", 100)
	_ = s.SetReaction(ctx, "w1", "bob", "❤", 200)
	rs, _ := s.ListReactions(ctx, "w1")
	if len(rs) != 2 {
		t.Errorf("two senders should coexist; got %+v", rs)
	}
}

func TestReactionsForChat_GroupsByTarget(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	_ = s.SetReaction(ctx, "w1", "alice", "👍", 100)
	_ = s.SetReaction(ctx, "w1", "bob", "❤", 110)
	_ = s.SetReaction(ctx, "w2", "alice", "🎉", 200)
	_ = s.SetReaction(ctx, "w3", "bob", "🤔", 300)

	groups, err := s.ReactionsForChat(ctx, []string{"w1", "w2", "w-missing"})
	if err != nil {
		t.Fatalf("ReactionsForChat: %v", err)
	}
	if len(groups["w1"]) != 2 || len(groups["w2"]) != 1 {
		t.Errorf("ReactionsForChat counts: %+v", groups)
	}
	if _, ok := groups["w3"]; ok {
		t.Errorf("w3 not requested; should not appear in result")
	}
}

func TestReactionsForChat_EmptyInputReturnsNil(t *testing.T) {
	s := openTempStore(t)
	groups, err := s.ReactionsForChat(context.Background(), nil)
	if err != nil {
		t.Fatalf("ReactionsForChat: %v", err)
	}
	if groups != nil {
		t.Errorf("nil input should return nil map; got %+v", groups)
	}
}

func TestSetReaction_RequiresArgs(t *testing.T) {
	s := openTempStore(t)
	if err := s.SetReaction(context.Background(), "", "alice", "👍", 1); err == nil {
		t.Error("empty target waID: want error")
	}
	if err := s.SetReaction(context.Background(), "w1", "", "👍", 1); err == nil {
		t.Error("empty sender: want error")
	}
}
