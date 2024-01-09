package lyra2

// cannotSubmitWorkError cannot submit a POW work
type cannotSubmitWorkError struct{ info string }

func (e cannotSubmitWorkError) ErrorCode() int { return -32005 }

func (e cannotSubmitWorkError) Error() string { return "Cannot submit work." }

func (e cannotSubmitWorkError) ErrorInfo() string { return e.info }
