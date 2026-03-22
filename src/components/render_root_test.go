package components

import "testing"

func TestUseSubmittedQRFullscreen(t *testing.T) {
	if !useSubmittedQRFullscreen(ViewState{
		Phase:     PhaseLog,
		Submitted: true,
		MobileQR:  []string{"██"},
	}) {
		t.Fatalf("expected fullscreen QR for submitted log phase with QR data")
	}

	if !useSubmittedQRFullscreen(ViewState{
		Phase:       PhaseLog,
		Submitted:   true,
		MobileQRErr: "qr unavailable",
	}) {
		t.Fatalf("expected fullscreen QR for submitted log phase with QR error")
	}

	if useSubmittedQRFullscreen(ViewState{
		Phase:     PhaseRemote,
		Submitted: true,
		MobileQR:  []string{"██"},
	}) {
		t.Fatalf("did not expect fullscreen QR outside log phase")
	}

	if useSubmittedQRFullscreen(ViewState{
		Phase:     PhaseLog,
		Submitted: true,
		Err:       "setup failed",
		MobileQR:  []string{"██"},
	}) {
		t.Fatalf("did not expect fullscreen QR when setup has an error")
	}
}
