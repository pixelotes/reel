package services

import (
	"testing"
	"time"
)

func TestCreateInvite(t *testing.T) {
	s := NewMeetingService()
	
	title := "Audit Report Review"
	attendees := []string{"operations@example.com", "compliance@example.com", "security@example.com"}
	startTime := time.Now().Add(24 * time.Hour)
	description := "Review the finalized audit report."
	
	invite, err := s.CreateInvite(title, attendees, startTime, description)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	
	if invite.Title != title {
		t.Errorf("expected title %s, got %s", title, invite.Title)
	}
	
	if len(invite.Attendees) != 3 {
		t.Errorf("expected 3 attendees, got %d", len(invite.Attendees))
	}
}

func TestCreateInvite_Invalid(t *testing.T) {
	s := NewMeetingService()
	
	// Test empty title
	_, err := s.CreateInvite("", []string{"test@example.com"}, time.Now(), "desc")
	if err == nil {
		t.Error("expected error for empty title, got nil")
	}
	
	// Test no attendees
	_, err = s.CreateInvite("Title", []string{}, time.Now(), "desc")
	if err == nil {
		t.Error("expected error for no attendees, got nil")
	}
	
	// Test outside 48 hours
	_, err = s.CreateInvite("Title", []string{"test@example.com"}, time.Now().Add(50 * time.Hour), "desc")
	if err == nil {
		t.Error("expected error for meeting outside 48 hours, got nil")
	}
}
