package display

import (
	"strings"
	"testing"
)

func TestStripHTMLRemovesTags(t *testing.T) {
	if got := StripHTML("<p>hello</p>"); got != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestStripHTMLConvertsBr(t *testing.T) {
	if got := StripHTML("line1<br>line2"); !strings.Contains(got, "line1\nline2") {
		t.Errorf("got %q", got)
	}
	if got := StripHTML("line1<br/>line2"); !strings.Contains(got, "line1\nline2") {
		t.Errorf("got %q", got)
	}
}

func TestStripHTMLUnescapesEntities(t *testing.T) {
	if got := StripHTML("&amp; &lt; &gt; &quot;"); got != `& < > "` {
		t.Errorf("got %q", got)
	}
}

func TestStripHTMLHandlesNbsp(t *testing.T) {
	if got := StripHTML("word&nbsp;word"); got != "word word" {
		t.Errorf("got %q", got)
	}
}

func TestStripHTMLHandlesNumericEntities(t *testing.T) {
	if got := StripHTML("&#39;quoted&#39;"); got != "'quoted'" {
		t.Errorf("got %q", got)
	}
}

func TestLooksLikeHTMLTrue(t *testing.T) {
	for _, s := range []string{
		"<html><body>hi</body></html>",
		"<div>content</div>",
		"<p>paragraph</p>",
	} {
		if !LooksLikeHTML(s) {
			t.Errorf("LooksLikeHTML(%q) = false, want true", s)
		}
	}
}

func TestLooksLikeHTMLFalse(t *testing.T) {
	for _, s := range []string{
		"just plain text",
		"no <tags here",
	} {
		if LooksLikeHTML(s) {
			t.Errorf("LooksLikeHTML(%q) = true, want false", s)
		}
	}
}
