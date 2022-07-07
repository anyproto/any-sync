package threadbuilder

type ThreadDescription struct {
	Author string `yaml:"author"`
}

type Keys struct {
	Enc  []string `yaml:"Enc"`
	Sign []string `yaml:"Sign"`
	Read []string `yaml:"Read"`
}

type ACLSnapshot struct {
	UserStates []struct {
		Identity          string   `yaml:"identity"`
		EncryptionKey     string   `yaml:"encryptionKey"`
		EncryptedReadKeys []string `yaml:"encryptedReadKeys"`
		Permissions       string   `yaml:"permission"`
		IsConfirmed       bool     `yaml:"isConfirmed"`
	} `yaml:"userStates"`
}

type PlainTextSnapshot struct {
	Text string `yaml:"text"`
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

type PlainTextChange struct {
	TextAppend *struct {
		Text string `yaml:"text"`
	} `yaml:"textAppend"`
}

type GraphNode struct {
	Id           string   `yaml:"id"`
	BaseSnapshot string   `yaml:"baseSnapshot"`
	AclSnapshot  string   `yaml:"aclSnapshot"`
	ACLHeads     []string `yaml:"aclHeads"`
	TreeHeads    []string `yaml:"treeHeads"`
}

type Change struct {
	Id       string `yaml:"id"`
	Identity string `yaml:"identity"`

	AclSnapshot *ACLSnapshot       `yaml:"aclSnapshot"`
	Snapshot    *PlainTextSnapshot `yaml:"snapshot"`
	AclChanges  []*ACLChange       `yaml:"aclChanges"`
	Changes     []*PlainTextChange `yaml:"changes"`

	ReadKey string `yaml:"readKey"`
}

type YMLThread struct {
	Description    *ThreadDescription `yaml:"thread"`
	Changes        []*Change          `yaml:"changes"`
	UpdatedChanges []*Change          `yaml:"updatedChanges"`

	Keys Keys `yaml:"keys"`

	Graph        []*GraphNode `yaml:"graph"`
	UpdatedGraph []*GraphNode `yaml:"updatedGraph"`

	Heads      []string `yaml:"heads"`
	MaybeHeads []string `yaml:"maybeHeads"`
}
