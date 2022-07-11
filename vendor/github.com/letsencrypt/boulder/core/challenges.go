package core

func newChallenge(challengeType AcmeChallenge, token string) Challenge {
	return Challenge{
		Type:   challengeType,
		Status: StatusPending,
		Token:  token,
	}
}

// HTTPChallenge01 constructs a random http-01 challenge. If token is empty a random token
// will be generated, otherwise the provided token is used.
func HTTPChallenge01(token string) Challenge {
	return newChallenge(ChallengeTypeHTTP01, token)
}

// DNSChallenge01 constructs a random dns-01 challenge. If token is empty a random token
// will be generated, otherwise the provided token is used.
func DNSChallenge01(token string) Challenge {
	return newChallenge(ChallengeTypeDNS01, token)
}

// TLSALPNChallenge01 constructs a random tls-alpn-01 challenge. If token is empty a random token
// will be generated, otherwise the provided token is used.
func TLSALPNChallenge01(token string) Challenge {
	return newChallenge(ChallengeTypeTLSALPN01, token)
}
