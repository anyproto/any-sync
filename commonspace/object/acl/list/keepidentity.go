package list

import (
	"errors"
	"io"

	protohelpers "github.com/planetscale/vtprotobuf/protohelpers"

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
	wireEndGroup                    = 4 // start/end-group (unused by these messages)
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
// Safe only on the trusted build path (caller gates on !verifier.ShouldValidate()); the record is still
// authenticated over its raw bytes by the caller, independent of this decode.
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
			out.MetadataPubKey = cloneBytes(body)
		case fieldRkcEncryptedMetadataPriv:
			if seenEncMeta {
				return nil, errNonCanonical
			}
			seenEncMeta = true
			out.EncryptedMetadataPrivKey = cloneBytes(body)
		case fieldRkcEncryptedOldReadKey:
			if seenOldKey {
				return nil, errNonCanonical
			}
			seenOldKey = true
			out.EncryptedOldReadKey = cloneBytes(body)
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
			out.Identities = append(out.Identities, cloneBytes(body))
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

// --- wire helpers ---

// readTag reads a field tag, rejecting illegal tags (field number <= 0) and group wire types the way the
// generated decoder does.
func readTag(dAtA []byte, i int) (field int32, wt int, ni int, err error) {
	tag, ni, err := readVarint(dAtA, i)
	if err != nil {
		return 0, 0, i, err
	}
	field = int32(tag >> 3)
	wt = int(tag & 0x7)
	if field <= 0 || wt == wireEndGroup || wt == 3 {
		return 0, 0, i, errNonCanonical
	}
	return field, wt, ni, nil
}

func readVarint(dAtA []byte, i int) (uint64, int, error) {
	var v uint64
	for shift := uint(0); ; shift += 7 {
		if shift >= 64 {
			return 0, i, protohelpers.ErrIntOverflow
		}
		if i >= len(dAtA) {
			return 0, i, io.ErrUnexpectedEOF
		}
		b := dAtA[i]
		i++
		v |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return v, i, nil
		}
	}
}

// readBytes reads a length-delimited field's payload as a subslice of dAtA (no copy) and returns the index
// past it.
func readBytes(dAtA []byte, i int) ([]byte, int, error) {
	n, ni, err := readVarint(dAtA, i)
	if err != nil {
		return nil, i, err
	}
	byteLen := int(n)
	if byteLen < 0 {
		return nil, i, protohelpers.ErrInvalidLength
	}
	end := ni + byteLen
	if end < 0 {
		return nil, i, protohelpers.ErrInvalidLength
	}
	if end > len(dAtA) {
		return nil, i, io.ErrUnexpectedEOF
	}
	return dAtA[ni:end], end, nil
}

// cloneBytes mirrors vtproto's `append(m.X[:0], data...)` for a present bytes field: a non-nil owned copy.
func cloneBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
