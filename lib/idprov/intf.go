package idprov

type IDProv interface {
	GetID() (string, error)
}
