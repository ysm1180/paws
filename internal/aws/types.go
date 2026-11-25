package aws

type Instance struct {
	ID            string
	Name          string
	Endpoint      string
	Port          int
	Type          string
	PrivateIP     string
	Description   string
	Platform      string
	SsmPingStatus string
	Region        string
}

// Key uniquely identifies the instance across regions. RDS/ElastiCache
// identifiers are user-chosen and can repeat in different regions, so
// session and config bookkeeping must not use the bare ID.
func (i Instance) Key() string {
	if i.Region == "" {
		return i.ID
	}
	return i.Region + "/" + i.ID
}
