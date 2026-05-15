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
}
