package strkey

const (
	AccountAddressVersionByte VersionByte = 0x5b // Base58-encodes to 'A...'
	AccountSeedVersionByte    VersionByte = 0xff // Base58-encodes to 'S...'
	DeviceSeedVersionByte     VersionByte = 0x7d // Base58-encodes to 'D...'
	NetworkAddressVersionByte VersionByte = 0xd3 // Base58-encodes to `N...`
)
