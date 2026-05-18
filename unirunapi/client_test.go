package api

import "testing"

func TestIsHTMLResponseDetectsWAFPages(t *testing.T) {
	cases := [][]byte{
		[]byte("<!doctype html><html><title>405</title></html>"),
		[]byte("<!doctypehtml><html lang=\"zh-cn\"><title>405</title></html>"),
		[]byte("Sorry, your request has been blocked as it may cause potential threats to the server's security"),
		[]byte("<script>var block_traceid='abc'</script>"),
		[]byte("https://errors.aliyun.com/static/image/405.png"),
	}

	for _, body := range cases {
		if !isHTMLResponse(body) {
			t.Fatalf("expected WAF/HTML response to be detected: %s", string(body))
		}
	}
}

func TestIsHTMLResponseAllowsJSON(t *testing.T) {
	if isHTMLResponse([]byte(`{"code":10000,"msg":"ok","response":{}}`)) {
		t.Fatal("JSON response should not be detected as HTML")
	}
}
