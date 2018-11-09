package config

import (
	"testing"

	"fmt"

	"github.com/google/certificate-transparency-go/trillian/ctfe/configpb"
)

func TestStructLogConfig(t *testing.T) {
	want := `configpb.LogConfig{LogId:0, Prefix:"", OverrideHandlerPrefix:"", RootsPemFile:[]string(nil), PrivateKey:(*any.Any)(nil), PublicKey:(*keyspb.PublicKey)(nil), RejectExpired:false, ExtKeyUsages:[]string(nil), NotAfterStart:(*timestamp.Timestamp)(nil), NotAfterLimit:(*timestamp.Timestamp)(nil), AcceptOnlyCa:false, LogBackendName:"", IsMirror:false, MaxMergeDelaySec:0, ExpectedMergeDelaySec:0, FrozenSth:(*configpb.SignedTreeHead)(nil), XXX_NoUnkeyedLiteral:struct {}{}, XXX_unrecognized:[]uint8(nil), XXX_sizecache:0}`
	cand := fmt.Sprintf("%#v", configpb.LogConfig{})

	if want != cand {
		t.Fatal("LogConfig struct has changed")
	}
}
