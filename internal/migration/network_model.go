package migration

type Network struct {
	ID       int64
	Name     string `db:"primary=yes"`
	Location string

	Config map[string]string `db:"marshal=json"`
}

func (n Network) Validate() error {
	if n.ID < 0 {
		return NewValidationErrf("Invalid network, id can not be negative")
	}

	if n.Name == "" {
		return NewValidationErrf("Invalid network, name can not be empty")
	}

	if n.Location == "" {
		return NewValidationErrf("Invalid network, location can not be empty")
	}

	return nil
}

type Networks []Network
