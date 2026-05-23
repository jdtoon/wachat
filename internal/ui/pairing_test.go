package ui

import (
	"testing"

	"github.com/jdtoon/wachat/internal/wa"
)

func TestPairingView_StartsInWaitingQR(t *testing.T) {
	p := NewPairingView()
	if p.Phase() != PairingWaitingQR {
		t.Errorf("initial Phase = %v, want PairingWaitingQR", p.Phase())
	}
}

func TestPairingView_CodeStoresPayloadAndStaysWaiting(t *testing.T) {
	p := NewPairingView()
	p.HandleQR(wa.QRItem{Event: "code", Code: "deadbeef"})
	if p.Phase() != PairingWaitingQR {
		t.Errorf("Phase = %v, want PairingWaitingQR", p.Phase())
	}
	if p.qrCode != "deadbeef" {
		t.Errorf("qrCode = %q, want %q", p.qrCode, "deadbeef")
	}
}

func TestPairingView_SuccessTransitionsToScanned(t *testing.T) {
	p := NewPairingView()
	p.HandleQR(wa.QRItem{Event: "success"})
	if p.Phase() != PairingScanned {
		t.Errorf("Phase = %v, want PairingScanned", p.Phase())
	}
}

func TestPairingView_TimeoutMovesToFailed(t *testing.T) {
	p := NewPairingView()
	p.HandleQR(wa.QRItem{Event: "timeout"})
	if p.Phase() != PairingFailed {
		t.Errorf("Phase = %v, want PairingFailed", p.Phase())
	}
	if p.errMsg == "" {
		t.Error("PairingFailed without errMsg")
	}
}

func TestPairingView_UnknownEventMovesToFailed(t *testing.T) {
	p := NewPairingView()
	p.HandleQR(wa.QRItem{Event: "err-pair"})
	if p.Phase() != PairingFailed {
		t.Errorf("Phase = %v, want PairingFailed", p.Phase())
	}
}

func TestPairingView_SetPhaseAndSetError(t *testing.T) {
	p := NewPairingView()
	p.SetPhase(PairingSyncing)
	if p.Phase() != PairingSyncing {
		t.Errorf("Phase = %v, want PairingSyncing", p.Phase())
	}
	p.SetError("nope")
	if p.Phase() != PairingFailed {
		t.Errorf("Phase after SetError = %v, want PairingFailed", p.Phase())
	}
}

// TestBannerCopy_ReturnsEmptyOnConnected guards the "no banner when
// everything is fine" invariant.
func TestBannerCopy_ReturnsEmptyOnConnected(t *testing.T) {
	th := NewTheme(LightPalette)
	msg, _, _ := bannerCopy(th, ConnConnected)
	if msg != "" {
		t.Errorf("ConnConnected banner copy = %q, want empty", msg)
	}
}

func TestBannerCopy_HasMessageForEachNonOKState(t *testing.T) {
	th := NewTheme(LightPalette)
	for _, s := range []ConnState{ConnConnecting, ConnDisconnected, ConnLoggedOut} {
		msg, _, _ := bannerCopy(th, s)
		if msg == "" {
			t.Errorf("ConnState %v missing banner copy", s)
		}
	}
}
