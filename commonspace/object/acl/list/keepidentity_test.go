package list

import (
	"bytes"

	"encoding/binary"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func byteMatch(want []byte) func([]byte) bool {
	return func(id []byte) bool { return bytes.Equal(id, want) }
}

func filterToIdentity(keys []*aclrecordproto.AclEncryptedReadKey, identity []byte) []*aclrecordproto.AclEncryptedReadKey {
	var out []*aclrecordproto.AclEncryptedReadKey
	for _, k := range keys {
		if bytes.Equal(k.Identity, identity) {
			out = append(out, k)
		}
	}
	return out
}

func appendLenDelim(dst []byte, field int, payload []byte) []byte {
	dst = binary.AppendUvarint(dst, uint64(field)<<3|wireBytes)
	dst = binary.AppendUvarint(dst, uint64(len(payload)))
	return append(dst, payload...)
}

func protoFieldCount(m interface{}) int {
	ty := reflect.TypeOf(m)
	n := 0
	for i := 0; i < ty.NumField(); i++ {
		if _, ok := ty.Field(i).Tag.Lookup("protobuf"); ok {
			n++
		}
	}
	return n
}

func erk(id, key string) *aclrecordproto.AclEncryptedReadKey {
	return &aclrecordproto.AclEncryptedReadKey{Identity: []byte(id), EncryptedReadKey: []byte(key)}
}

func requireSameEncKeys(t *testing.T, want, got []*aclrecordproto.AclEncryptedReadKey) {
	require.Len(t, got, len(want))
	for i := range want {
		require.Equal(t, want[i].Identity, got[i].Identity)
		require.Equal(t, want[i].EncryptedReadKey, got[i].EncryptedReadKey)
	}
}

func TestKeepIdentity_EquivalentToFullDecodeExceptForeignAccountKeys(t *testing.T) {
	mine := []byte("me")
	full := &aclrecordproto.AclData{
		AclContent: []*aclrecordproto.AclContentValue{
			{Value: &aclrecordproto.AclContentValue_ReadKeyChange{ReadKeyChange: &aclrecordproto.AclReadKeyChange{
				AccountKeys: []*aclrecordproto.AclEncryptedReadKey{
					erk("alice", "ka"),
					{Identity: mine, EncryptedReadKey: []byte("kme")},
					erk("bob", "kb"),
				},
				MetadataPubKey:           []byte("mpub"),
				EncryptedMetadataPrivKey: []byte("empriv"),
				EncryptedOldReadKey:      []byte("eork"),
				InviteKeys:               []*aclrecordproto.AclEncryptedReadKey{erk("inv1", "ik1"), erk("inv2", "ik2")},
			}}},
			{Value: &aclrecordproto.AclContentValue_AccountRemove{AccountRemove: &aclrecordproto.AclAccountRemove{
				Identities: [][]byte{[]byte("alice"), []byte("bob")},
				ReadKeyChange: &aclrecordproto.AclReadKeyChange{
					AccountKeys: []*aclrecordproto.AclEncryptedReadKey{
						erk("carol", "kc"),
						{Identity: mine, EncryptedReadKey: []byte("kme2")},
					},
					MetadataPubKey:           []byte("mpub2"),
					EncryptedMetadataPrivKey: []byte("empriv2"),
					EncryptedOldReadKey:      []byte("eork2"),
				},
			}}},
			// a non-readKeyChange content: must pass through identical to the full decode.
			{Value: &aclrecordproto.AclContentValue_PermissionChange{PermissionChange: &aclrecordproto.AclAccountPermissionChange{
				Identity:    []byte("dave"),
				Permissions: aclrecordproto.AclUserPermissions_Writer,
			}}},
		},
	}

	raw, err := full.MarshalVT()
	require.NoError(t, err)

	mineDecoded, err := unmarshalAclDataKeepIdentity(raw, byteMatch(mine))
	require.NoError(t, err)
	require.Len(t, mineDecoded.AclContent, 3)

	// content[0] readKeyChange: accountKeys filtered to ours; everything else intact.
	rkc := mineDecoded.AclContent[0].GetReadKeyChange()
	require.NotNil(t, rkc)
	requireSameEncKeys(t, []*aclrecordproto.AclEncryptedReadKey{{Identity: mine, EncryptedReadKey: []byte("kme")}}, rkc.AccountKeys)
	require.Equal(t, []byte("mpub"), rkc.MetadataPubKey)
	require.Equal(t, []byte("empriv"), rkc.EncryptedMetadataPrivKey)
	require.Equal(t, []byte("eork"), rkc.EncryptedOldReadKey)
	requireSameEncKeys(t, []*aclrecordproto.AclEncryptedReadKey{erk("inv1", "ik1"), erk("inv2", "ik2")}, rkc.InviteKeys)

	// content[1] accountRemove: identities intact; nested readKeyChange filtered to ours.
	ar := mineDecoded.AclContent[1].GetAccountRemove()
	require.NotNil(t, ar)
	require.Equal(t, [][]byte{[]byte("alice"), []byte("bob")}, ar.Identities)
	requireSameEncKeys(t, []*aclrecordproto.AclEncryptedReadKey{{Identity: mine, EncryptedReadKey: []byte("kme2")}}, ar.ReadKeyChange.AccountKeys)
	require.Equal(t, []byte("mpub2"), ar.ReadKeyChange.MetadataPubKey)
	require.Equal(t, []byte("eork2"), ar.ReadKeyChange.EncryptedOldReadKey)

	// content[2] permissionChange: identical to the full decode (pass-through path).
	pc := mineDecoded.AclContent[2].GetPermissionChange()
	require.NotNil(t, pc)
	require.Equal(t, []byte("dave"), pc.Identity)
	require.Equal(t, aclrecordproto.AclUserPermissions_Writer, pc.Permissions)
}

func TestKeepIdentity_NoMatchYieldsEmptyAccountKeys(t *testing.T) {
	full := &aclrecordproto.AclData{AclContent: []*aclrecordproto.AclContentValue{
		{Value: &aclrecordproto.AclContentValue_ReadKeyChange{ReadKeyChange: &aclrecordproto.AclReadKeyChange{
			AccountKeys:         []*aclrecordproto.AclEncryptedReadKey{erk("alice", "ka"), erk("bob", "kb")},
			EncryptedOldReadKey: []byte("eork"),
		}}},
	}}
	raw, err := full.MarshalVT()
	require.NoError(t, err)
	got, err := unmarshalAclDataKeepIdentity(raw, byteMatch([]byte("stranger")))
	require.NoError(t, err)
	require.Empty(t, got.AclContent[0].GetReadKeyChange().AccountKeys)
	require.Equal(t, []byte("eork"), got.AclContent[0].GetReadKeyChange().EncryptedOldReadKey)
}

// TestKeepIdentity_FieldNumbers pins the wire field numbers the partial decoder hard-codes against what the
// generated marshaller actually emits. A single-field message marshals to exactly one tag, so its first byte
// is (fieldNumber<<3 | wireType). If a .proto renumber changes any of these, this test fails loudly.
func TestKeepIdentity_FieldNumbers(t *testing.T) {
	firstTag := func(m interface{ MarshalVT() ([]byte, error) }) byte {
		b, err := m.MarshalVT()
		require.NoError(t, err)
		require.NotEmpty(t, b)
		return b[0]
	}
	tag := func(field, wire int) byte { return byte(field<<3 | wire) }

	require.Equal(t, tag(fieldAclDataContent, wireBytes),
		firstTag(&aclrecordproto.AclData{AclContent: []*aclrecordproto.AclContentValue{{Value: &aclrecordproto.AclContentValue_ReadKeyChange{ReadKeyChange: &aclrecordproto.AclReadKeyChange{}}}}}))
	require.Equal(t, tag(fieldContentReadKeyChange, wireBytes),
		firstTag(&aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_ReadKeyChange{ReadKeyChange: &aclrecordproto.AclReadKeyChange{}}}))
	require.Equal(t, tag(fieldContentAccountRemove, wireBytes),
		firstTag(&aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_AccountRemove{AccountRemove: &aclrecordproto.AclAccountRemove{}}}))
	require.Equal(t, tag(fieldRkcAccountKeys, wireBytes),
		firstTag(&aclrecordproto.AclReadKeyChange{AccountKeys: []*aclrecordproto.AclEncryptedReadKey{erk("a", "b")}}))
	require.Equal(t, tag(fieldRkcMetadataPubKey, wireBytes), firstTag(&aclrecordproto.AclReadKeyChange{MetadataPubKey: []byte("x")}))
	require.Equal(t, tag(fieldRkcEncryptedMetadataPriv, wireBytes), firstTag(&aclrecordproto.AclReadKeyChange{EncryptedMetadataPrivKey: []byte("x")}))
	require.Equal(t, tag(fieldRkcEncryptedOldReadKey, wireBytes), firstTag(&aclrecordproto.AclReadKeyChange{EncryptedOldReadKey: []byte("x")}))
	require.Equal(t, tag(fieldRkcInviteKeys, wireBytes),
		firstTag(&aclrecordproto.AclReadKeyChange{InviteKeys: []*aclrecordproto.AclEncryptedReadKey{erk("a", "b")}}))
	require.Equal(t, tag(fieldAccountRemoveIdentities, wireBytes), firstTag(&aclrecordproto.AclAccountRemove{Identities: [][]byte{[]byte("x")}}))
	require.Equal(t, tag(fieldAccountRemoveReadKeyChange, wireBytes), firstTag(&aclrecordproto.AclAccountRemove{ReadKeyChange: &aclrecordproto.AclReadKeyChange{}}))
	require.Equal(t, tag(fieldEncReadKeyIdentity, wireBytes), firstTag(&aclrecordproto.AclEncryptedReadKey{Identity: []byte("x")}))
}

// TestKeepIdentity_RoundTripVsFullDecode is the strong equivalence guard: keep-only-ours must equal a REAL
// aclrecordproto.AclData.UnmarshalVT of the same bytes with accountKeys filtered to ours. Comparing marshalled bytes means a
// future .proto field that the hand-rolled decoder forgets to handle (and therefore drops) shows up as a diff.
func TestKeepIdentity_RoundTripVsFullDecode(t *testing.T) {
	mine := []byte("me")
	full := &aclrecordproto.AclData{AclContent: []*aclrecordproto.AclContentValue{
		{Value: &aclrecordproto.AclContentValue_ReadKeyChange{ReadKeyChange: &aclrecordproto.AclReadKeyChange{
			AccountKeys:         []*aclrecordproto.AclEncryptedReadKey{erk("alice", "ka"), {Identity: mine, EncryptedReadKey: []byte("kme")}, erk("bob", "kb")},
			MetadataPubKey:      []byte("mpub"),
			EncryptedOldReadKey: []byte("eork"),
			InviteKeys:          []*aclrecordproto.AclEncryptedReadKey{erk("inv", "ik")},
		}}},
		{Value: &aclrecordproto.AclContentValue_AccountRemove{AccountRemove: &aclrecordproto.AclAccountRemove{
			Identities:    [][]byte{[]byte("alice")},
			ReadKeyChange: &aclrecordproto.AclReadKeyChange{AccountKeys: []*aclrecordproto.AclEncryptedReadKey{erk("carol", "kc"), {Identity: mine, EncryptedReadKey: []byte("kme2")}}},
		}}},
		{Value: &aclrecordproto.AclContentValue_PermissionChange{PermissionChange: &aclrecordproto.AclAccountPermissionChange{Identity: []byte("dave"), Permissions: aclrecordproto.AclUserPermissions_Writer}}},
	}}
	raw, err := full.MarshalVT()
	require.NoError(t, err)

	mineDecoded, err := unmarshalAclDataKeepIdentity(raw, byteMatch(mine))
	require.NoError(t, err)

	ref := &aclrecordproto.AclData{}
	require.NoError(t, ref.UnmarshalVT(raw))
	for _, c := range ref.AclContent {
		if rkc := c.GetReadKeyChange(); rkc != nil {
			rkc.AccountKeys = filterToIdentity(rkc.AccountKeys, mine)
		}
		if ar := c.GetAccountRemove(); ar != nil && ar.ReadKeyChange != nil {
			ar.ReadKeyChange.AccountKeys = filterToIdentity(ar.ReadKeyChange.AccountKeys, mine)
		}
	}
	gotBytes, err := mineDecoded.MarshalVT()
	require.NoError(t, err)
	wantBytes, err := ref.MarshalVT()
	require.NoError(t, err)
	require.Equal(t, wantBytes, gotBytes, "keep-only-ours diverged from full-decode-with-accountKeys-filtered (a field was dropped or added?)")
}

// TestKeepIdentity_FieldCountPins fails loudly if a field is ADDED to any message the hand-rolled allowlist
// decoder handles — otherwise the new field would be silently dropped on the keep-only-ours path while full
// decode keeps it. When this fails: handle the new field in the decoder, then bump the count here.
func TestKeepIdentity_FieldCountPins(t *testing.T) {
	require.Equal(t, 1, protoFieldCount(aclrecordproto.AclData{}), "aclrecordproto.AclData fields changed -> update keepIdentityFast")
	require.Equal(t, 5, protoFieldCount(aclrecordproto.AclReadKeyChange{}), "aclrecordproto.AclReadKeyChange fields changed -> update keepReadKeyChange")
	require.Equal(t, 2, protoFieldCount(aclrecordproto.AclEncryptedReadKey{}), "aclrecordproto.AclEncryptedReadKey fields changed -> update encryptedReadKeyMatches")
	require.Equal(t, 2, protoFieldCount(aclrecordproto.AclAccountRemove{}), "aclrecordproto.AclAccountRemove fields changed -> update keepAccountRemove")
}

// TestKeepIdentity_NonCanonicalOneofMatchesFullDecode verifies the oneof fallback: a non-canonical
// aclrecordproto.AclContentValue carrying two oneof variants must resolve to the SAME variant as the generated full decoder
// (last-wins), not whichever the partial decoder happens to read first.
func TestKeepIdentity_NonCanonicalOneofMatchesFullDecode(t *testing.T) {
	rkcEmpty, err := (&aclrecordproto.AclReadKeyChange{}).MarshalVT()
	require.NoError(t, err)
	arEmpty, err := (&aclrecordproto.AclAccountRemove{}).MarshalVT()
	require.NoError(t, err)
	var cv []byte
	cv = appendLenDelim(cv, fieldContentReadKeyChange, rkcEmpty) // field 7 first
	cv = appendLenDelim(cv, fieldContentAccountRemove, arEmpty)  // field 6 last -> last-wins
	var data []byte
	data = appendLenDelim(data, fieldAclDataContent, cv)

	full := &aclrecordproto.AclData{}
	require.NoError(t, full.UnmarshalVT(data))
	mine, err := unmarshalAclDataKeepIdentity(data, byteMatch([]byte("x")))
	require.NoError(t, err)

	require.Equal(t, full.AclContent[0].GetReadKeyChange() != nil, mine.AclContent[0].GetReadKeyChange() != nil, "readKeyChange variant must agree")
	require.Equal(t, full.AclContent[0].GetAccountRemove() != nil, mine.AclContent[0].GetAccountRemove() != nil, "accountRemove variant must agree")
}

// dupIdentityElement builds aclrecordproto.AclData->readKeyChange->accountKeys[0] = {identity: decoy, identity: mine, key}.
// The generated decoder is last-wins (identity=mine); a first-wins peek would drop our entry.
func dupIdentityElement(mine []byte) []byte {
	var elem []byte
	elem = appendLenDelim(elem, fieldEncReadKeyIdentity, []byte("decoy"))
	elem = appendLenDelim(elem, fieldEncReadKeyIdentity, mine)
	elem = appendLenDelim(elem, fieldEncReadKeyEncryptedKey, []byte("k"))
	rkc := appendLenDelim(nil, fieldRkcAccountKeys, elem)
	cv := appendLenDelim(nil, fieldContentReadKeyChange, rkc)
	return appendLenDelim(nil, fieldAclDataContent, cv)
}

// splitAccountRemove builds aclrecordproto.AclData->accountRemove with TWO readKeyChange field-2 submessages (A holds our
// key, B holds metadata). The generated decoder MERGES them; an overwrite would drop our key.
func splitAccountRemove(mine []byte) []byte {
	var elem []byte
	elem = appendLenDelim(elem, fieldEncReadKeyIdentity, mine)
	elem = appendLenDelim(elem, fieldEncReadKeyEncryptedKey, []byte("k"))
	rkcA := appendLenDelim(nil, fieldRkcAccountKeys, elem)
	rkcB := appendLenDelim(nil, fieldRkcMetadataPubKey, []byte("m"))
	var ar []byte
	ar = appendLenDelim(ar, fieldAccountRemoveReadKeyChange, rkcA)
	ar = appendLenDelim(ar, fieldAccountRemoveReadKeyChange, rkcB)
	cv := appendLenDelim(nil, fieldContentAccountRemove, ar)
	return appendLenDelim(nil, fieldAclDataContent, cv)
}

// FuzzKeepIdentity is the safety gate: for ANY input, the keep-only-ours decode must equal the authoritative
// full decode + filter. Because unmarshalAclDataKeepIdentity falls back to fullDecodeFilter on any fast-path
// anomaly, a divergence here means the fast path accepted something it should not have.
// Run: go test -run x -fuzz FuzzKeepIdentity ./commonspace/object/acl/list/
func FuzzKeepIdentity(f *testing.F) {
	mine := []byte("me")
	match := byteMatch(mine)
	seed := func(d *aclrecordproto.AclData) {
		if b, err := d.MarshalVT(); err == nil {
			f.Add(b)
		}
	}
	seed(&aclrecordproto.AclData{AclContent: []*aclrecordproto.AclContentValue{{Value: &aclrecordproto.AclContentValue_ReadKeyChange{ReadKeyChange: &aclrecordproto.AclReadKeyChange{
		AccountKeys:    []*aclrecordproto.AclEncryptedReadKey{erk("a", "ka"), {Identity: mine, EncryptedReadKey: []byte("k")}, erk("b", "kb")},
		MetadataPubKey: []byte("m"), EncryptedOldReadKey: []byte("o"), InviteKeys: []*aclrecordproto.AclEncryptedReadKey{erk("i", "ik")},
	}}}}})
	seed(&aclrecordproto.AclData{AclContent: []*aclrecordproto.AclContentValue{{Value: &aclrecordproto.AclContentValue_AccountRemove{AccountRemove: &aclrecordproto.AclAccountRemove{
		Identities: [][]byte{[]byte("a")}, ReadKeyChange: &aclrecordproto.AclReadKeyChange{AccountKeys: []*aclrecordproto.AclEncryptedReadKey{{Identity: mine, EncryptedReadKey: []byte("k")}}},
	}}}}})
	f.Add(dupIdentityElement(mine))
	f.Add(splitAccountRemove(mine))
	f.Add([]byte{0x08, 0x00}) // aclrecordproto.AclData field 1 as a varint (wrong wire type)

	f.Fuzz(func(t *testing.T, data []byte) {
		got, gErr := unmarshalAclDataKeepIdentity(data, match)
		ref, rErr := fullDecodeFilter(data, match)
		require.Equal(t, rErr == nil, gErr == nil, "error-ness must agree with full decode")
		if gErr != nil {
			return
		}
		gb, err := got.MarshalVT()
		require.NoError(t, err)
		rb, err := ref.MarshalVT()
		require.NoError(t, err)
		require.Equal(t, rb, gb, "keep-only-ours diverged from full-decode+filter")
	})
}

func TestKeepIdentity_DuplicateIdentityKeepsOurs(t *testing.T) {
	mine := []byte("me")
	data := dupIdentityElement(mine)
	got, err := unmarshalAclDataKeepIdentity(data, byteMatch(mine))
	require.NoError(t, err)
	require.Len(t, got.AclContent[0].GetReadKeyChange().AccountKeys, 1, "last-wins identity == mine -> our entry kept")
	ref, err := fullDecodeFilter(data, byteMatch(mine))
	require.NoError(t, err)
	gb, _ := got.MarshalVT()
	rb, _ := ref.MarshalVT()
	require.Equal(t, rb, gb)
}

func TestKeepIdentity_SplitAccountRemoveKeepsOurs(t *testing.T) {
	mine := []byte("me")
	data := splitAccountRemove(mine)
	got, err := unmarshalAclDataKeepIdentity(data, byteMatch(mine))
	require.NoError(t, err)
	require.Len(t, got.AclContent[0].GetAccountRemove().ReadKeyChange.AccountKeys, 1, "merged submessages -> our key kept")
	ref, err := fullDecodeFilter(data, byteMatch(mine))
	require.NoError(t, err)
	gb, _ := got.MarshalVT()
	rb, _ := ref.MarshalVT()
	require.Equal(t, rb, gb)
}

// TestKeepIdentity_SemanticMatch: an identity that is byte-different but matches via the closure (modelling a
// non-canonically-encoded pubkey) must keep our entry — a raw bytes.Equal would silently drop it.
func TestKeepIdentity_SemanticMatch(t *testing.T) {
	onWire := []byte("ME")
	match := func(id []byte) bool { return bytes.EqualFold(id, []byte("me")) }
	full := &aclrecordproto.AclData{AclContent: []*aclrecordproto.AclContentValue{{Value: &aclrecordproto.AclContentValue_ReadKeyChange{ReadKeyChange: &aclrecordproto.AclReadKeyChange{
		AccountKeys: []*aclrecordproto.AclEncryptedReadKey{{Identity: onWire, EncryptedReadKey: []byte("k")}, erk("bob", "kb")},
	}}}}}
	raw, err := full.MarshalVT()
	require.NoError(t, err)
	got, err := unmarshalAclDataKeepIdentity(raw, match)
	require.NoError(t, err)
	require.Len(t, got.AclContent[0].GetReadKeyChange().AccountKeys, 1, "semantic match must keep our entry despite byte-different identity")
	require.Equal(t, onWire, got.AclContent[0].GetReadKeyChange().AccountKeys[0].Identity)
}
