package threadmodels

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/lib/core/smartblock"
	"testing"
)

func TestCreateACLThreadIDVerify(t *testing.T) {
	_, pubKey, err := GenerateRandomEd25519KeyPair()
	if err != nil {
		t.Fatalf("should not return error after generating key pair: %v", err)
	}

	thread, err := CreateACLThreadID(pubKey, smartblock.SmartBlockTypeWorkspace)
	if err != nil {
		t.Fatalf("should not return error after generating thread: %v", err)
	}

	verified, err := VerifyACLThreadID(pubKey, thread)
	if err != nil {
		t.Fatalf("verification should not return error: %v", err)
	}

	if !verified {
		t.Fatalf("the thread should be verified")
	}
}
