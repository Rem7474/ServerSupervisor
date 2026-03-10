package events

// CommandCompletionListener reacts to a remote command terminal state update.
type CommandCompletionListener interface {
	HandleCommandCompletion(commandID, status string)
}
