package hooks

type PreSave interface {
	PreSave() interface{}
}
