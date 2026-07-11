package list

import (
	"bytes"
	"errors"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
)

// Wire field numbers the partial decoder depends on. Pinned by TestKeepIdentity_FieldNumbers /
// TestKeepIdentity_FieldCountPins so a .proto renumber or new field fails loudly.
const (
	fieldAclDataContent             = 1 // AclData.aclContent (repeated AclContentValue)
	fieldContentReadKeyChange       = 7 // AclContentValue.readKeyChange
	fieldContentAccountRemove       = 6 // AclContentValue.accountRemove
	fieldRkcAccountKeys             = 1 // AclReadKeyChange.accountKeys (repeated AclEncryptedReadKey)
	fieldRkcMetadataPubKey          = 2 // AclReadKeyChange.metadataPubKey
	fieldRkcEncryptedMetadataPriv   = 3 // AclReadKeyChange.encryptedMetadataPrivKey
	fieldRkcEncryptedOldReadKey     = 4 // AclReadKeyChange.encryptedOldReadKey
	fieldRkcInviteKeys              = 5 // AclReadKeyChange.inviteKeys (repeated AclEncryptedReadKey)
	fieldAccountRemoveIdentities    = 1 // AclAccountRemove.identities (repeated bytes)
	fieldAccountRemoveReadKeyChange = 2 // AclAccountRemove.readKeyChange
	fieldEncReadKeyIdentity         = 1 // AclEncryptedReadKey.identity
	fieldEncReadKeyEncryptedKey     = 2 // AclEncryptedReadKey.encryptedReadKey
	wireBytes                       = 2 // length-delimited wire type
)

// errNonCanonical signals that the strict fast path hit input it does not handle exactly the way the
// generated decoder would (unknown/duplicate field, wrong wire type, illegal tag, trailing bytes, a split
// submessage, a non-read-key content). The caller then defers to the authoritative full decode.
var errNonCanonical = errors.New("keep-identity: non-canonical input")

// unmarshalAclDataKeepIdentity decodes AclData, keeping inside every read key change (standalone, and the one
// nested in an AclAccountRemove) only the AclEncryptedReadKey for which `isOurs` returns true, and skipping
// every other member's at the wire level (never allocated). applyReadKeyChange reads only our own entry, so
// the rest is dead weight.
//
// It is a pure optimization of fullDecodeFilter: a STRICT fast path handles only canonical, single-content
// read-key records, and on ANY anomaly defers to AclData.UnmarshalVT + an in-place filter — so its output can
// never diverge from the generated decoder (verified exhaustively by FuzzKeepIdentity). `isOurs` must mirror
// the consumer's semantic identity comparison (see aclRecordBuilder.isOurIdentity), not a raw byte compare.
//
// Runs wherever the verifier does not validate content (caller gates on !verifier.ShouldValidate()): the
// build-from-storage path and live ingest via AddRawRecord alike. The record is still authenticated over its
// raw bytes by the caller, independent of this decode, and a non-validating verifier never inspects other
// members' accountKeys — but the resulting Model is a shrunken view of the wire content.
func unmarshalAclDataKeepIdentity(dAtA []byte, isOurs func(identity []byte) bool) (*aclrecordproto.AclData, error) {
	if out, err := keepIdentityFast(dAtA, isOurs); err == nil {
		return out, nil
	}
	return fullDecodeFilter(dAtA, isOurs)
}

// fullDecodeFilter is the authoritative path: the generated decoder, then accountKeys filtered to `isOurs` in
// place. Used as the fallback and as the fuzz oracle.
func fullDecodeFilter(dAtA []byte, isOurs func(identity []byte) bool) (*aclrecordproto.AclData, error) {
	data := &aclrecordproto.AclData{}
	if err := data.UnmarshalVT(dAtA); err != nil {
		return nil, err
	}
	for _, c := range data.AclContent {
		if rkc := c.GetReadKeyChange(); rkc != nil {
			rkc.AccountKeys = filterAccountKeys(rkc.AccountKeys, isOurs)
		}
		if ar := c.GetAccountRemove(); ar != nil && ar.ReadKeyChange != nil {
			ar.ReadKeyChange.AccountKeys = filterAccountKeys(ar.ReadKeyChange.AccountKeys, isOurs)
		}
	}
	return data, nil
}

func filterAccountKeys(keys []*aclrecordproto.AclEncryptedReadKey, isOurs func(identity []byte) bool) []*aclrecordproto.AclEncryptedReadKey {
	var out []*aclrecordproto.AclEncryptedReadKey
	for _, k := range keys {
		if isOurs(k.Identity) {
			out = append(out, k)
		}
	}
	return out
}

func keepIdentityFast(dAtA []byte, isOurs func(identity []byte) bool) (*aclrecordproto.AclData, error) {
	out := &aclrecordproto.AclData{}
	i, l := 0, len(dAtA)
	for i < l {
		field, wt, ni, err := readTag(dAtA, i)
		if err != nil {
			return nil, err
		}
		i = ni
		if field != fieldAclDataContent || wt != wireBytes {
			return nil, errNonCanonical
		}
		cv, ni2, err := readBytes(dAtA, i)
		if err != nil {
			return nil, err
		}
		i = ni2
		content, err := keepContentValue(cv, isOurs)
		if err != nil {
			return nil, err
		}
		out.AclContent = append(out.AclContent, content)
	}
	return out, nil
}

// keepContentValue takes the fast path only for a canonical oneof — exactly one field spanning the whole
// buffer — that is a readKeyChange or accountRemove. Everything else (other variants, extra fields, a oneof
// not spanning the buffer) defers to the full decode; non-read-key variants carry no fan-out, so nothing is
// lost.
func keepContentValue(cv []byte, isOurs func(identity []byte) bool) (*aclrecordproto.AclContentValue, error) {
	field, wt, ni, err := readTag(cv, 0)
	if err != nil {
		return nil, err
	}
	if wt != wireBytes {
		return nil, errNonCanonical
	}
	body, next, err := readBytes(cv, ni)
	if err != nil {
		return nil, err
	}
	if next != len(cv) {
		return nil, errNonCanonical
	}
	switch field {
	case fieldContentReadKeyChange:
		rkc, err := keepReadKeyChange(body, isOurs)
		if err != nil {
			return nil, err
		}
		return &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_ReadKeyChange{ReadKeyChange: rkc}}, nil
	case fieldContentAccountRemove:
		ar, err := keepAccountRemove(body, isOurs)
		if err != nil {
			return nil, err
		}
		return &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_AccountRemove{AccountRemove: ar}}, nil
	default:
		return nil, errNonCanonical
	}
}

func keepReadKeyChange(dAtA []byte, isOurs func(identity []byte) bool) (*aclrecordproto.AclReadKeyChange, error) {
	out := &aclrecordproto.AclReadKeyChange{}
	var seenMeta, seenEncMeta, seenOldKey bool
	i, l := 0, len(dAtA)
	for i < l {
		field, wt, ni, err := readTag(dAtA, i)
		if err != nil {
			return nil, err
		}
		i = ni
		if wt != wireBytes {
			return nil, errNonCanonical
		}
		body, ni2, err := readBytes(dAtA, i)
		if err != nil {
			return nil, err
		}
		i = ni2
		switch field {
		case fieldRkcAccountKeys:
			keep, err := encryptedReadKeyMatches(body, isOurs)
			if err != nil {
				return nil, err
			}
			if keep {
				ek := &aclrecordproto.AclEncryptedReadKey{}
				if err := ek.UnmarshalVT(body); err != nil {
					return nil, err
				}
				out.AccountKeys = append(out.AccountKeys, ek)
			}
		case fieldRkcMetadataPubKey:
			if seenMeta {
				return nil, errNonCanonical
			}
			seenMeta = true
			out.MetadataPubKey = bytes.Clone(body)
		case fieldRkcEncryptedMetadataPriv:
			if seenEncMeta {
				return nil, errNonCanonical
			}
			seenEncMeta = true
			out.EncryptedMetadataPrivKey = bytes.Clone(body)
		case fieldRkcEncryptedOldReadKey:
			if seenOldKey {
				return nil, errNonCanonical
			}
			seenOldKey = true
			out.EncryptedOldReadKey = bytes.Clone(body)
		case fieldRkcInviteKeys:
			ek := &aclrecordproto.AclEncryptedReadKey{}
			if err := ek.UnmarshalVT(body); err != nil {
				return nil, err
			}
			out.InviteKeys = append(out.InviteKeys, ek)
		default:
			return nil, errNonCanonical
		}
	}
	return out, nil
}

func keepAccountRemove(dAtA []byte, isOurs func(identity []byte) bool) (*aclrecordproto.AclAccountRemove, error) {
	out := &aclrecordproto.AclAccountRemove{}
	var seenReadKeyChange bool
	i, l := 0, len(dAtA)
	for i < l {
		field, wt, ni, err := readTag(dAtA, i)
		if err != nil {
			return nil, err
		}
		i = ni
		if wt != wireBytes {
			return nil, errNonCanonical
		}
		body, ni2, err := readBytes(dAtA, i)
		if err != nil {
			return nil, err
		}
		i = ni2
		switch field {
		case fieldAccountRemoveIdentities:
			out.Identities = append(out.Identities, bytes.Clone(body))
		case fieldAccountRemoveReadKeyChange:
			if seenReadKeyChange {
				return nil, errNonCanonical // the generated decoder MERGES split submessages; we cannot, so defer
			}
			seenReadKeyChange = true
			rkc, err := keepReadKeyChange(body, isOurs)
			if err != nil {
				return nil, err
			}
			out.ReadKeyChange = rkc
		default:
			return nil, errNonCanonical
		}
	}
	return out, nil
}

// encryptedReadKeyMatches strictly validates an AclEncryptedReadKey element (single identity, single key, no
// unknown fields, no trailing bytes) and reports whether its identity makes `isOurs` return true. Because it
// walks the whole element, skipped elements are still structurally validated, and a duplicate identity field
// forces the fallback rather than a first-vs-last-wins divergence.
func encryptedReadKeyMatches(elem []byte, isOurs func(identity []byte) bool) (bool, error) {
	var identity []byte
	var seenIdentity, seenKey bool
	i, l := 0, len(elem)
	for i < l {
		field, wt, ni, err := readTag(elem, i)
		if err != nil {
			return false, err
		}
		i = ni
		if wt != wireBytes {
			return false, errNonCanonical
		}
		body, ni2, err := readBytes(elem, i)
		if err != nil {
			return false, err
		}
		i = ni2
		switch field {
		case fieldEncReadKeyIdentity:
			if seenIdentity {
				return false, errNonCanonical
			}
			seenIdentity = true
			identity = body
		case fieldEncReadKeyEncryptedKey:
			if seenKey {
				return false, errNonCanonical
			}
			seenKey = true
		default:
			return false, errNonCanonical
		}
	}
	return isOurs(identity), nil
}

// --- wire helpers (thin wrappers over protowire; exact error values don't matter, any error defers to the
// authoritative full decode) ---

// readTag reads a field tag, rejecting illegal tags (field number <= 0, via protowire) and group wire types
// the way the generated decoder does.
func readTag(dAtA []byte, i int) (field int32, wt int, ni int, err error) {
	num, typ, n := protowire.ConsumeTag(dAtA[i:])
	if n < 0 || typ == protowire.StartGroupType || typ == protowire.EndGroupType {
		return 0, 0, i, errNonCanonical
	}
	return int32(num), int(typ), i + n, nil
}

// readBytes reads a length-delimited field's payload as a subslice of dAtA (no copy) and returns the index
// past it.
func readBytes(dAtA []byte, i int) ([]byte, int, error) {
	b, n := protowire.ConsumeBytes(dAtA[i:])
	if n < 0 {
		return nil, i, errNonCanonical
	}
	return b, i + n, nil
}
