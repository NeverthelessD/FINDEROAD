package main

// macOS 는 Finder 에서 만든 한글 파일 이름을 자모가 나뉜 형태(NFD)로 저장하는 경우가 많아요.
// 검색어(보통 NFC)와 맞춰 보려면 둘 다 같은 형태로 바꿔야 해요. (표준 라이브러리만 쓰려고 직접 구현)

const (
	sBase  = 0xAC00
	lBase  = 0x1100
	vBase  = 0x1161
	tBase  = 0x11A7
	lCount = 19
	vCount = 21
	tCount = 28
	nCount = vCount * tCount
	sCount = lCount * nCount
)

// composeHangul: 나뉜 자모(ㅎ+ㅏ+ㄴ)를 완성형 글자(한)로 합쳐요. 한글이 아닌 글자는 그대로 둬요.
func composeHangul(s string) string {
	need := false
	for _, r := range s {
		if r >= 0x1100 && r <= 0x11FF {
			need = true
			break
		}
	}
	if !need {
		return s
	}
	rs := []rune(s)
	out := make([]rune, 0, len(rs))
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r >= lBase && r < lBase+lCount && i+1 < len(rs) && rs[i+1] >= vBase && rs[i+1] < vBase+vCount {
			syl := sBase + ((r-lBase)*vCount+(rs[i+1]-vBase))*tCount
			i++
			if i+1 < len(rs) && rs[i+1] > tBase && rs[i+1] < tBase+tCount {
				syl += rs[i+1] - tBase
				i++
			}
			out = append(out, syl)
			continue
		}
		// 이미 합쳐진 받침 없는 글자 + 받침
		if r >= sBase && r < sBase+sCount && (r-sBase)%tCount == 0 && i+1 < len(rs) && rs[i+1] > tBase && rs[i+1] < tBase+tCount {
			out = append(out, r+(rs[i+1]-tBase))
			i++
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

// decomposeHangul: 완성형 글자(한)를 자모(ㅎ+ㅏ+ㄴ)로 나눠요. Spotlight 검색에서 두 형태를 모두 찾으려고 써요.
func decomposeHangul(s string) string {
	need := false
	for _, r := range s {
		if r >= sBase && r < sBase+sCount {
			need = true
			break
		}
	}
	if !need {
		return s
	}
	out := make([]rune, 0, len(s)*2)
	for _, r := range s {
		if r >= sBase && r < sBase+sCount {
			idx := r - sBase
			out = append(out, lBase+idx/nCount, vBase+(idx%nCount)/tCount)
			if t := idx % tCount; t != 0 {
				out = append(out, tBase+t)
			}
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
