package command

type Registry interface {
	Register(cmd Command) error
	Get(name string) (Command, bool)
	List() []Command
}
