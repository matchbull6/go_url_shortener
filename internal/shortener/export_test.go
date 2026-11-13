package shortener

var (
	CheckURL     = checkURL
	GenerateCode = generateCode
)

const (
	Chars         = chars
	MaxAttempts   = maxAttempts
	MaxURLLength  = maxURLLength
	MinCodeLength = minCodeLength
	MaxCodeLength = maxCodeLength
)

func SetCodeGenerator(s *Service, f func() (string, error)) {
	s.newCode = f
}
