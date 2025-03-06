package nria

type EntityID int64

// Fingerprint is used in the agent connect step when communicating with the backend. Based on it
// the backend will uniquely identify the agent and respond with the entityKey and entityId.
type Fingerprint struct {
	FullHostname    string    `json:"fullHostname"`
	Hostname        string    `json:"hostname"`
	CloudProviderId string    `json:"cloudProviderId"`
	DisplayName     string    `json:"displayName"`
	BootID       string    `json:"bootId"`
	IpAddresses  Addresses `json:"ipAddresses"`
	MacAddresses Addresses `json:"macAddresses"`
}

// Addresses will store the nic addresses mapped by the nickname.
type Addresses map[string][]string

// Equals check to see if current fingerprint is equal to provided one.
func (f *Fingerprint) Equals(new Fingerprint) bool {
	return f.Hostname == new.Hostname &&
		f.FullHostname == new.FullHostname &&
		f.CloudProviderId == new.CloudProviderId &&
		f.BootID == new.BootID &&
		f.DisplayName == new.DisplayName &&
		f.IpAddresses.Equals(new.IpAddresses) &&
		f.MacAddresses.Equals(new.MacAddresses)
}

// Equals check if the Address has the same values.
func (a Addresses) Equals(b Addresses) bool {
	if (a == nil) != (b == nil) {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for keyA, valA := range a {
		valB, exists := b[keyA]
		if !exists {
			return false
		}
		if len(valA) != len(valB) {
			return false
		}

		for i := range valA {
			if valA[i] != valB[i] {
				return false
			}
		}
	}
	return true
}



