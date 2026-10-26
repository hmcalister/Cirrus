package main

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/hmcalister/cirrus/internal/config"
	"github.com/hmcalister/skysatchel/pkg/email/skysender"
)

func TestNextSendTime(t *testing.T) {
	at := config.DailyEmailConfig{Hour: 8, Minute: 0}
	sameDay := time.Date(2026, time.October, 26, 8, 0, 0, 0, time.UTC)
	nextDay := sameDay.AddDate(0, 0, 1)

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"before the send time", sameDay.Add(-30 * time.Minute), sameDay},
		{"at the send time", sameDay, nextDay},
		{"after the send time", sameDay.Add(time.Hour), nextDay},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := nextSendTime(tc.now, at); !got.Equal(tc.want) {
				t.Errorf("nextSendTime(%v) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
}

type fakeUsers struct {
	emails []string
	err    error
}

func (f fakeUsers) ListActiveEmails(context.Context) ([]string, error) {
	return f.emails, f.err
}

// An in-memory FileGetter. Files are kept in a map so tests need not care
// about names beyond the one being attached.
type fakeFileGetter struct {
	files     map[string][]byte
	deleted   []string
	listErr   error
	getErr    error
	listCalls int
}

func (f *fakeFileGetter) ListFiles(context.Context) ([]string, error) {
	f.listCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	names := make([]string, 0, len(f.files))
	for name := range f.files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (f *fakeFileGetter) GetFile(_ context.Context, name string) ([]byte, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.files[name], nil
}

func (f *fakeFileGetter) DeleteFile(_ context.Context, name string) error {
	f.deleted = append(f.deleted, name)
	delete(f.files, name)
	return nil
}

// Captures the messages it is asked to send, and fails the calls it is told to.
type fakeSender struct {
	sent        []skysender.SendParams
	calls       int
	errsForCall map[int]error
}

func (f *fakeSender) Send(_ context.Context, params skysender.SendParams) error {
	call := f.calls
	f.calls++
	if err := f.errsForCall[call]; err != nil {
		return err
	}
	f.sent = append(f.sent, params)
	return nil
}

func newTestEmailer(users userLister, files *fakeFileGetter, sender *fakeSender, batchSize int) dailyEmailer {
	return dailyEmailer{
		users:       users,
		files:       files,
		sender:      sender,
		fromAddress: "noreply@example.com",
		batchSize:   batchSize,
	}
}

func TestSendDailyEmailsSendsOneCloudToBatches(t *testing.T) {
	emails := []string{
		"one@example.com",
		"two@example.com",
		"three@example.com",
		"four@example.com",
		"five@example.com",
	}
	images := map[string][]byte{
		"clouds/one.jpg":   []byte("one"),
		"clouds/two.jpg":   []byte("two"),
		"clouds/three.jpg": []byte("three"),
	}
	files := &fakeFileGetter{files: maps.Clone(images)}
	sender := &fakeSender{}
	daily := newTestEmailer(fakeUsers{emails: emails}, files, sender, 2)

	if err := daily.send(context.Background()); err != nil {
		t.Fatalf("send() err = %v", err)
	}

	wantBatchSizes := []int{2, 2, 1}
	if len(sender.sent) != len(wantBatchSizes) {
		t.Fatalf("sent %d messages, want %d", len(sender.sent), len(wantBatchSizes))
	}

	if len(files.deleted) != 1 {
		t.Fatalf("deleted %d images, want 1", len(files.deleted))
	}
	name := files.deleted[0]
	wantImage := images[name]

	var received []string
	for i, params := range sender.sent {
		if got := len(params.BCCRecipients); got != wantBatchSizes[i] {
			t.Errorf("batch %d has %d BCC recipients, want %d", i, got, wantBatchSizes[i])
		}
		received = append(received, params.BCCRecipients...)

		if got := params.ToRecipients; len(got) != 1 || got[0] != "noreply@example.com" {
			t.Errorf("batch %d To recipients = %v", i, got)
		}

		if len(params.Attachments) != 1 {
			t.Fatalf("batch %d has %d attachments, want 1", i, len(params.Attachments))
		}
		attachment := params.Attachments[0]
		if want := filepath.Base(name); attachment.Filename != want {
			t.Errorf("batch %d attachment = %q, want %q", i, attachment.Filename, want)
		}
		if !bytes.Equal(attachment.Data, wantImage) {
			t.Errorf("batch %d attachment data = %q, want %q", i, attachment.Data, wantImage)
		}
	}

	if !slices.Equal(received, emails) {
		t.Errorf("BCC recipients = %v, want %v", received, emails)
	}
	if len(files.files) != len(images)-1 {
		t.Errorf("store has %d files, want %d", len(files.files), len(images)-1)
	}
}

func TestSendDailyEmailsSkipsRunWithoutActiveUsers(t *testing.T) {
	files := &fakeFileGetter{files: map[string][]byte{"clouds/today.jpg": []byte("cloud")}}
	sender := &fakeSender{}
	daily := newTestEmailer(fakeUsers{}, files, sender, 10)

	if err := daily.send(context.Background()); err != nil {
		t.Fatalf("send() err = %v", err)
	}

	if len(sender.sent) != 0 {
		t.Errorf("sent %d messages, want none", len(sender.sent))
	}
	if files.listCalls != 0 {
		t.Errorf("listed images %d times, want none", files.listCalls)
	}
	if len(files.deleted) != 0 {
		t.Errorf("deleted = %v, want none", files.deleted)
	}
}

func TestSendDailyEmailsFailsWithoutImages(t *testing.T) {
	files := &fakeFileGetter{files: map[string][]byte{}}
	sender := &fakeSender{}
	daily := newTestEmailer(fakeUsers{emails: []string{"user@example.com"}}, files, sender, 10)

	err := daily.send(context.Background())
	if err == nil {
		t.Fatal("send() err = nil, want error")
	}

	if len(sender.sent) != 0 {
		t.Errorf("sent %d messages, want none", len(sender.sent))
	}
	if len(files.deleted) != 0 {
		t.Errorf("deleted = %v, want none", files.deleted)
	}
}

func TestSendDailyEmailsKeepsImageWhenGetFails(t *testing.T) {
	files := &fakeFileGetter{
		files:  map[string][]byte{"clouds/today.jpg": []byte("cloud")},
		getErr: errors.New("bucket is unavailable"),
	}
	sender := &fakeSender{}
	daily := newTestEmailer(fakeUsers{emails: []string{"user@example.com"}}, files, sender, 10)

	err := daily.send(context.Background())
	if err == nil {
		t.Fatal("send() err = nil, want error")
	}

	if len(sender.sent) != 0 {
		t.Errorf("sent %d messages, want none", len(sender.sent))
	}
	if len(files.deleted) != 0 {
		t.Errorf("deleted = %v, want none", files.deleted)
	}
}

func TestSendDailyEmailsKeepsImageWhenEveryBatchFails(t *testing.T) {
	files := &fakeFileGetter{files: map[string][]byte{"clouds/today.jpg": []byte("cloud")}}
	sender := &fakeSender{errsForCall: map[int]error{
		0: errors.New("smtp is down"),
		1: errors.New("smtp is down"),
	}}
	emails := []string{"one@example.com", "two@example.com", "three@example.com"}
	daily := newTestEmailer(fakeUsers{emails: emails}, files, sender, 2)

	if err := daily.send(context.Background()); err != nil {
		t.Fatalf("send() err = %v", err)
	}

	if len(files.deleted) != 0 {
		t.Errorf("deleted = %v, want none", files.deleted)
	}
	if _, ok := files.files["clouds/today.jpg"]; !ok {
		t.Error("image was removed from the store")
	}
}

func TestSendDailyEmailsDeletesImageWhenOneBatchSucceeds(t *testing.T) {
	files := &fakeFileGetter{files: map[string][]byte{"clouds/today.jpg": []byte("cloud")}}
	sender := &fakeSender{errsForCall: map[int]error{0: errors.New("smtp is down")}}
	emails := []string{"one@example.com", "two@example.com", "three@example.com", "four@example.com"}
	daily := newTestEmailer(fakeUsers{emails: emails}, files, sender, 2)

	if err := daily.send(context.Background()); err != nil {
		t.Fatalf("send() err = %v", err)
	}

	if len(sender.sent) != 1 {
		t.Errorf("sent %d messages, want 1", len(sender.sent))
	}
	if len(files.deleted) != 1 {
		t.Errorf("deleted %d images, want 1", len(files.deleted))
	}
}
