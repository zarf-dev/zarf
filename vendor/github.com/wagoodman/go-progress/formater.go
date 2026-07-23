package progress

type Formatter func(Progress) (string, error)
