package idprov

import "github.com/google/uuid"

type RandomID struct {
	IDLen int16
}

func ProvideRandomID(IDLen int16) *RandomID {
	return &RandomID{
		IDLen: IDLen,
	}
}

func (p *RandomID) GetID() (string, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	return id.String()[0:p.IDLen], nil
}
