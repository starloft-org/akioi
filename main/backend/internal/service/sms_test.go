package service

import "testing"

func TestValidPhone(t *testing.T) {
	for _, phone := range []string{"13800138000", "19912345678"} {
		if !ValidPhone(phone) {
			t.Errorf("expected valid phone: %s", phone)
		}
	}
	for _, phone := range []string{"", "12800138000", "1380013800", "1380013800x"} {
		if ValidPhone(phone) {
			t.Errorf("expected invalid phone: %s", phone)
		}
	}
}

func TestStarLoftSign(t *testing.T) {
	got := starLoftSign("POST", "/v2/sms/send", "", "1700000000", "0123456789abcdef", []byte(`{"a":1}`), "secret")
	const want = "e0a348aaae3908a12e51e6a04a1be21ed601c5944e207b905491145bf3eaa266"
	if got != want {
		t.Fatalf("signature mismatch: got %s want %s", got, want)
	}
}
