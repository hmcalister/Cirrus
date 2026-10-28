package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/hmcalister/skysatchel/pkg/email/skysender"
)

type fakeUsers struct {
	emails      []string
	adminEmails []string
	err         error
	adminErr    error
	adminCalls  int
}

func (f *fakeUsers) ListActiveEmails(context.Context) ([]string, error) {
	return f.emails, f.err
}

func (f *fakeUsers) ListAdminEmails(context.Context) ([]string, error) {
	f.adminCalls++
	return f.adminEmails, f.adminErr
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
	daily := newTestEmailer(&fakeUsers{emails: emails}, files, sender, 2)

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
	daily := newTestEmailer(&fakeUsers{}, files, sender, 10)

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
	daily := newTestEmailer(&fakeUsers{emails: []string{"user@example.com"}}, files, sender, 10)

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
	daily := newTestEmailer(&fakeUsers{emails: []string{"user@example.com"}}, files, sender, 10)

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
	daily := newTestEmailer(&fakeUsers{emails: emails}, files, sender, 2)

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

func TestSendDailyEmailsAlertsAdminsWhenImagesRunLow(t *testing.T) {
	tests := []struct {
		name       string
		imageCount int
		wantAlert  bool
	}{
		{"seven remain", 8, true},
		{"eight remain", 9, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			images := make(map[string][]byte, tc.imageCount)
			for i := range tc.imageCount {
				images[fmt.Sprintf("clouds/%d.jpg", i)] = []byte("cloud")
			}

			files := &fakeFileGetter{files: images}
			sender := &fakeSender{}
			users := &fakeUsers{
				emails:      []string{"user@example.com"},
				adminEmails: []string{"admin@example.com"},
			}
			daily := newTestEmailer(users, files, sender, 10)

			if err := daily.send(context.Background()); err != nil {
				t.Fatalf("send() err = %v", err)
			}

			wantMessages := 1
			wantAdminCalls := 0
			if tc.wantAlert {
				wantMessages = 2
				wantAdminCalls = 1
			}
			if len(sender.sent) != wantMessages {
				t.Fatalf("sent %d messages, want %d", len(sender.sent), wantMessages)
			}
			if users.adminCalls != wantAdminCalls {
				t.Errorf("listed admins %d times, want %d", users.adminCalls, wantAdminCalls)
			}
			if !tc.wantAlert {
				return
			}

			alert := sender.sent[1]
			if want := []string{"admin@example.com"}; !slices.Equal(alert.BCCRecipients, want) {
				t.Errorf("alert BCC recipients = %v, want %v", alert.BCCRecipients, want)
			}
			if len(alert.Attachments) != 0 {
				t.Errorf("alert has %d attachments, want none", len(alert.Attachments))
			}
			if !strings.Contains(alert.Body, "7") {
				t.Errorf("alert body = %q, want the remaining count", alert.Body)
			}
		})
	}
}

func TestSendDailyEmailsIgnoresFailedAdminAlert(t *testing.T) {
	images := map[string][]byte{
		"clouds/one.jpg": []byte("one"),
		"clouds/two.jpg": []byte("two"),
	}
	files := &fakeFileGetter{files: images}
	sender := &fakeSender{errsForCall: map[int]error{1: errors.New("smtp is down")}}
	users := &fakeUsers{
		emails:      []string{"user@example.com"},
		adminEmails: []string{"admin@example.com"},
	}
	daily := newTestEmailer(users, files, sender, 10)

	if err := daily.send(context.Background()); err != nil {
		t.Fatalf("send() err = %v, want nil", err)
	}

	if len(files.deleted) != 1 {
		t.Errorf("deleted %d images, want 1", len(files.deleted))
	}
	if len(sender.sent) != 1 {
		t.Errorf("sent %d messages, want 1", len(sender.sent))
	}
}

func TestSendDailyEmailsDeletesImageWhenOneBatchSucceeds(t *testing.T) {
	files := &fakeFileGetter{files: map[string][]byte{"clouds/today.jpg": []byte("cloud")}}
	sender := &fakeSender{errsForCall: map[int]error{0: errors.New("smtp is down")}}
	emails := []string{"one@example.com", "two@example.com", "three@example.com", "four@example.com"}
	daily := newTestEmailer(&fakeUsers{emails: emails}, files, sender, 2)

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
