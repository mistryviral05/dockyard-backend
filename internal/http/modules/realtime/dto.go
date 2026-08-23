package realtime

// connectionClaims Centrifugo ke connection JWT ka payload hai. Sirf "sub"
// chahiye: channel ki permission subscribe proxy decide karta hai, token nahi.
type connectionClaims struct {
	Sub string `json:"sub"`
}

// ConnectionResponse wo sab hai jo frontend ko connect karne ke liye chahiye.
type ConnectionResponse struct {
	Token     string `json:"token"`
	Url       string `json:"url"`
	ExpiresIn int64  `json:"expires_in"`
}

// subscribeRequest wo body hai jo Centrifugo hume bhejta hai jab koi client
// channel pe subscribe karne ki koshish karta hai.
type subscribeRequest struct {
	Client    string `json:"client"`
	Transport string `json:"transport"`
	User      string `json:"user"` // connection token ka "sub"
	Channel   string `json:"channel"`
}

// Centrifugo proxy ka jawab hamesha HTTP 200 hota hai. Allow ya deny body me
// jaata hai - non-200 ko wo apni internal error maanta hai.
type subscribeResponse struct {
	Result *subscribeResult `json:"result,omitempty"`
	Error  *proxyError      `json:"error,omitempty"`
}

type subscribeResult struct{}

type proxyError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
