package main

import (
	"errors"
	"testing"
)

func TestIsAPTLockError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "dpkg frontend lock",
			err:  errors.New("E: Could not get lock /var/lib/dpkg/lock-frontend. It is held by process 4820 (unattended-upgr)"),
			want: true,
		},
		{
			name: "apt lists lock",
			err:  errors.New("E: Unable to lock directory /var/lib/apt/lists/"),
			want: true,
		},
		{
			name: "other apt failure",
			err:  errors.New("E: Unable to locate package wireguard"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAPTLockError(tc.err); got != tc.want {
				t.Fatalf("isAPTLockError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestRunWithAPTRetry_RetriesLockErrors(t *testing.T) {
	attempts := 0
	out, err := runWithAPTRetry(func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("E: Could not get lock /var/lib/dpkg/lock-frontend. It is held by process 4820 (unattended-upgr)")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("runWithAPTRetry: %v", err)
	}
	if out != "ok" {
		t.Fatalf("unexpected output: %q", out)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRunWithAPTRetry_DoesNotRetryNonLockErrors(t *testing.T) {
	attempts := 0
	wantErr := errors.New("E: Unable to locate package wireguard")
	_, err := runWithAPTRetry(func() (string, error) {
		attempts++
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}
