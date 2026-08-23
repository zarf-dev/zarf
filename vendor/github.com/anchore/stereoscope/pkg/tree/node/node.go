package node

type Node interface {
	ID() ID
	Copy() Node
}
