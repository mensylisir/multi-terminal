package session

type State int

const (
    StateActive    State = iota
    StateDetached
    StateExpired
    StateClosed
)

func (s State) String() string {
    switch s {
    case StateActive:
        return "active"
    case StateDetached:
        return "detached"
    case StateExpired:
        return "expired"
    case StateClosed:
        return "closed"
    default:
        return "unknown"
    }
}