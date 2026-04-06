package unilog

import "fmt"

type OverflowPolicy int

const (
	OverflowDropNewest OverflowPolicy = iota
	OverflowBlock
)

func (p OverflowPolicy) String() string {
	switch p {
	case OverflowDropNewest:
		return "drop_newest"
	case OverflowBlock:
		return "block"
	default:
		return fmt.Sprintf("overflow_policy(%d)", int(p))
	}
}

func ParseOverflowPolicy(s string) (OverflowPolicy, error) {
	switch normalizeName(s) {
	case "", "drop_newest", "drop-newest":
		return OverflowDropNewest, nil
	case "block":
		return OverflowBlock, nil
	default:
		return OverflowDropNewest, fmt.Errorf("unilog: unsupported overflow policy %q", s)
	}
}
