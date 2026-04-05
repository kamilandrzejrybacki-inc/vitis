package claudecode

import "regexp"

var (
	blockedPromptPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\((y|yes)/(n|no)\)`),
		regexp.MustCompile(`(?i)\[(y|yes)/(n|no)\]`),
		regexp.MustCompile(`(?i)\b(press enter|press any key)\b`),
		regexp.MustCompile(`(?i)\b(continue\?|overwrite\?|are you sure\?)`),
		regexp.MustCompile(`(?i)\b(do you|would you|shall i|ready to)\b.*\?$`),
	}
	authPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(log in|login|sign in|authenticate|authentication required)\b`),
		regexp.MustCompile(`(?i)\b(oauth|browser.*open|api key.*required)\b`),
	}
	rateLimitPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)you've hit your`),
		regexp.MustCompile(`(?i)\b(rate limit|usage limit|session limit|weekly limit)\b`),
		regexp.MustCompile(`(?i)\b(out of extra usage|too many requests)\b`),
	}
	permissionPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(permission|approval|approve|deny|allow this)\b`),
	}
	promptReappearancePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?m)^>\s*$`),
		regexp.MustCompile(`(?m)^›\s*$`),
		regexp.MustCompile(`(?m)^claude>\s*$`),
	}
)
