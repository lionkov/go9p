package p

import (
	"testing"
)

func TestUnpack_BufferTooShortHeader(t *testing.T) {
	t.Parallel()
	_, err, used := Unpack([]byte{1, 2, 3, 4, 5, 6}, false)
	if err == nil {
		t.Fatalf("expected error")
	}
	if used != 0 {
		t.Fatalf("expected used=0, got %d", used)
	}
}

func TestUnpack_SizeFieldTooSmall(t *testing.T) {
	t.Parallel()
	// size=6 (< 7), type=Tversion, tag=0
	buf := []byte{
		6, 0, 0, 0,
		Tversion,
		0, 0,
	}
	_, err, used := Unpack(buf, false)
	if err == nil {
		t.Fatalf("expected error")
	}
	if used != 0 {
		t.Fatalf("expected used=0, got %d", used)
	}
}

func TestUnpack_SizeFieldBiggerThanBuffer(t *testing.T) {
	t.Parallel()
	// size=100, but buffer shorter; type=Tversion, tag=0
	buf := []byte{
		100, 0, 0, 0,
		Tversion,
		0, 0,
	}
	_, err, used := Unpack(buf, false)
	if err == nil {
		t.Fatalf("expected error")
	}
	if used != 0 {
		t.Fatalf("expected used=0, got %d", used)
	}
}

func TestUnpack_InvalidMessageType(t *testing.T) {
	t.Parallel()
	// size=7 (header only), type=99 (invalid; < Tversion)
	buf := []byte{
		7, 0, 0, 0,
		99,
		0, 0,
	}
	_, err, used := Unpack(buf, false)
	if err == nil {
		t.Fatalf("expected error")
	}
	if used != 0 {
		t.Fatalf("expected used=0, got %d", used)
	}
}

func TestUnpack_Tversion_TruncatedBody(t *testing.T) {
	t.Parallel()
	fc := NewFcall(64)
	if err := PackTversion(fc, 8192, "9P2000"); err != nil {
		t.Fatalf("pack: %v", err)
	}
	// Drop last byte => string length claims more than available.
	trunc := append([]byte(nil), fc.Pkt[:len(fc.Pkt)-1]...)
	_, err, used := Unpack(trunc, false)
	if err == nil {
		t.Fatalf("expected error")
	}
	if used != 0 {
		t.Fatalf("expected used=0, got %d", used)
	}
}

func TestUnpack_Rread_CountExceedsAvailable(t *testing.T) {
	t.Parallel()
	// Build a minimal Rread with count=10 but only 1 byte payload.
	buf := []byte{
		12, 0, 0, 0, // size = 7 + 4 + 1
		Rread,
		0, 0, // tag
		10, 0, 0, 0, // count=10
		0xAA, // only 1 byte data
	}
	_, err, used := Unpack(buf, false)
	if err == nil {
		t.Fatalf("expected error")
	}
	if used != 0 {
		t.Fatalf("expected used=0, got %d", used)
	}
}

func TestUnpack_Twalk_NameCountTooLarge(t *testing.T) {
	t.Parallel()
	// Twalk: fid + newfid + nwname=1, but missing the name string.
	buf := []byte{
		17, 0, 0, 0, // size = 7 + 4 + 4 + 2
		Twalk,
		0, 0, // tag
		1, 0, 0, 0, // fid
		2, 0, 0, 0, // newfid
		1, 0, // nwname=1 (but no name bytes follow)
	}
	_, err, used := Unpack(buf, false)
	if err == nil {
		t.Fatalf("expected error")
	}
	if used != 0 {
		t.Fatalf("expected used=0, got %d", used)
	}
}

func TestUnpack_ExtraTrailingBytes(t *testing.T) {
	t.Parallel()
	// Rflush is header-only (size=7). Add an extra byte but claim size=8.
	buf := []byte{
		8, 0, 0, 0,
		Rflush,
		0, 0,
		0xFF, // trailing junk
	}
	_, err, used := Unpack(buf, false)
	if err == nil {
		t.Fatalf("expected error")
	}
	if used != 0 {
		t.Fatalf("expected used=0, got %d", used)
	}
}

