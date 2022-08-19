package acllistbuilder

type Keys struct {
	Enc  []string `yaml:"Enc"`
	Sign []string `yaml:"Sign"`
	Read []string `yaml:"Read"`
}

type ACLChange struct {
	UserAdd *struct {
		Identity          string   `yaml:"identity"`
		EncryptionKey     string   `yaml:"encryptionKey"`
		EncryptedReadKeys []string `yaml:"encryptedReadKeys"`
		Permission        string   `yaml:"permission"`
	} `yaml:"userAdd"`

	UserJoin *struct {
		Identity          string   `yaml:"identity"`
		EncryptionKey     string   `yaml:"encryptionKey"`
		AcceptSignature   string   `yaml:"acceptSignature"`
		InviteId          string   `yaml:"inviteId"`
		EncryptedReadKeys []string `yaml:"encryptedReadKeys"`
	} `yaml:"userJoin"`

	UserInvite *struct {
		AcceptKey         string   `yaml:"acceptKey"`
		EncryptionKey     string   `yaml:"encryptionKey"`
		EncryptedReadKeys []string `yaml:"encryptedReadKeys"`
		Permissions       string   `yaml:"permissions"`
		InviteId          string   `yaml:"inviteId"`
	} `yaml:"userInvite"`

	UserConfirm *struct {
		Identity  string `yaml:"identity"`
		UserAddId string `yaml:"UserAddId"`
	} `yaml:"userConfirm"`

	UserRemove *struct {
		RemovedIdentity string   `yaml:"removedIdentity"`
		NewReadKey      string   `yaml:"newReadKey"`
		IdentitiesLeft  []string `yaml:"identitiesLeft"`
	} `yaml:"userRemove"`

	UserPermissionChange *struct {
		Identity   string `yaml:"identity"`
		Permission string `yaml:"permission"`
	}
}

type Record struct {
	Identity   string       `yaml:"identity"`
	AclChanges []*ACLChange `yaml:"aclChanges"`

	ReadKey string `yaml:"readKey"`
}

type Header struct {
	FirstChangeId string `yaml:"firstChangeId"`
	IsWorkspace   bool   `yaml:"isWorkspace"`
}

type YMLList struct {
	Records []*Record `yaml:"records"`

	Keys Keys `yaml:"keys"`
}
