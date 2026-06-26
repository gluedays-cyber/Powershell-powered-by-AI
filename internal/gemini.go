package internal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var ApiKey string

func getApiURL(model string) string {
	return "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":generateContent?key=" + ApiKey
}

func getStreamApiURL(model string) string {
	return "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":streamGenerateContent?alt=sse&key=" + ApiKey
}

// CallGeminiStream? SSE ?ㅽ듃由щ컢?쇰줈 Gemini ?묐떟??諛쏆븘 w??利됱떆 異쒕젰?섍퀬,
// ?꾩꽦???꾩껜 ?묐떟 臾몄옄?댁쓣 諛섑솚?쒕떎.
func CallGeminiStream(model string, history []Content, w io.Writer) (string, error) {
	nowTime := time.Now().Format("2006-01-02 15:04:05 (MST)")

	reqBody := GenerateRequest{
		Contents: ManageHistory(history),
		SystemInstruction: &SystemInstruction{
			Parts: []Part{{Text: buildSystemPrompt(nowTime)}},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", getStreamApiURL(model), bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status: %s, body: %s", resp.Status, string(bodyBytes))
	}

	var fullText strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		// SSE ?뺤떇: "data: {...}" ?쇱씤留?泥섎━
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		var chunk GenerateResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Candidates) == 0 || len(chunk.Candidates[0].Content.Parts) == 0 {
			continue
		}
		token := chunk.Candidates[0].Content.Parts[0].Text
		fullText.WriteString(token)
		if w != nil {
			fmt.Fprint(w, token)
		}
	}
	if err := scanner.Err(); err != nil {
		return fullText.String(), err
	}

	return fullText.String(), nil
}

type Content struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text"`
}

type GenerateRequest struct {
	Contents          []Content          `json:"contents"`
	SystemInstruction *SystemInstruction `json:"system_instruction,omitempty"`
}

type SystemInstruction struct {
	Parts []Part `json:"parts"`
}

type GenerateResponse struct {
	Candidates []Candidate `json:"candidates"`
}

type Candidate struct {
	Content Content `json:"content"`
}

func ManageHistory(history []Content) []Content {
	const maxThreshold = 8
	if len(history) <= maxThreshold {
		return history
	}

	managed := make([]Content, 0, maxThreshold)
	// 理쒖큹 ?ъ슜???붿껌 蹂댁〈
	managed = append(managed, history[0])

	// 理쒖큹 AI ?묐떟??蹂댁〈?섏뿬 珥덇린 ?먮쫫 ?좎?
	if len(history) > 1 && history[1].Role == "model" {
		managed = append(managed, history[1])
	}

	// 理쒓렐 ????댁슜 ?꾩＜濡??щ씪?대뵫 ?덈룄??援ъ꽦
	startIdx := len(history) - 4
	if startIdx < len(managed) {
		startIdx = len(managed)
	}

	for i := startIdx; i < len(history); i++ {
		managed = append(managed, history[i])
	}

	return managed
}

// buildSystemPrompt? ?쒖뒪???꾨＼?꾪듃 臾몄옄?댁쓣 ?앹꽦?쒕떎.
func buildSystemPrompt(nowTime string) string {
	return fmt.Sprintf("[?꾩옱 湲곗? ?쒓컙 ?뺣낫]\n?ㅻ뒛 ?좎쭨 諛??꾩옱 ?쒓컖: %s\n\n", nowTime) +
		"?뱀떊? ?ъ슜?먯쓽 ?붽뎄?ы빆???앸퀎?섏뿬 ?곹빀??媛쒕컻 ?몄뼱濡??묐룞 ?ㅽ겕由쏀듃???꾨줈洹몃옩???묒꽦???ㅽ뻾?섍굅?? ?ㅽ뻾 ?놁씠 吏곸젒 ?묐떟?????덈뒗 吏?ν삎 媛쒕컻 蹂댁“ AI??\n" +
		"肄붾뱶瑜??묒꽦???뚯쓽 ?몄뼱 ?좏깮 ?곗꽑?쒖쐞???ㅼ쓬怨?媛숈쑝硫? 諛섎뱶???대? 以?섑빐???쒕떎:\n" +
		"1?쒖쐞: Python (python ?먮뒗 py, ?뺤옣??.py)\n" +
		"2?쒖쐞: Go (go, ?뺤옣??.go)\n\n" +
		"?ъ슜?먯쓽 ?붽뎄?ы빆???닿껐?섍린 ?꾪빐 ?꾨옒 2媛吏 諛⑹떇 以??섎굹瑜??ㅼ뒪濡??먮떒?섏뿬 ?좏깮?댁빞 ?쒕떎:\n" +
		"1. 肄붾뱶濡??ㅽ뻾?댁빞 ?섎뒗 ?곹솴: ???곗꽑?쒖쐞???곕씪 ?곸젅???꾨줈洹몃옒諛??몄뼱瑜??좏깮?섏뿬 諛섎뱶??```?몄뼱紐?... ``` ?뺥깭??留덊겕?ㅼ슫 肄붾뱶 釉붾줉?쇰줈留??뚯뒪 肄붾뱶瑜??쒓났?댁빞 ?쒕떎. ?대떦 肄붾뱶???⑥씪 ?뚯씪濡??숈옉 媛?ν븳 ?뷀듃由ы룷?명듃瑜?援ъ꽦?댁빞 ?쒕떎.\n" +
		"2. 肄붾뱶 ?ㅽ뻾 ?놁씠 ??듭씠 媛?ν븷 ?? ?쇰컲 ?띿뒪???듬??쇰줈 吏곸젒 ??듯븳??\n\n" +
		"[?쒓컙 諛???꾩〈 ?뺣낫 ?뺤씤 吏移?\n" +
		"留뚯빟 ?ъ슜?먭? ?꾩옱 ?쒓컙, ?좎쭨, ?뱀? ?뱀젙 吏??쓽 ??꾩〈 愿???뺣낫瑜??붽뎄?섍굅???대떦 ?뺣낫媛 ?곗궛??以묒슂?섍쾶 ?쒖슜?쒕떎硫? ?대? 吏?앹씠???쒖뒪???대윮???좊ː?섏? 留먭퀬 諛섎뱶???꾨옒 ?뚯씠???⑥닔 'fetch_time_from_web'???몄텧?섏뿬 ??API濡쒕????숈쟻?쇰줈 ?섏쭛???쒓컙 ?뺣낫瑜?湲곗??쇰줈 ?듬???援ъ꽦?댁빞 ?쒕떎. 留뚯빟 API ?묒냽 ?ㅻ쪟 諛쒖깮 ???덉쇅 泥섎━濡?濡쒖뺄 ?쒖뒪?쒖쓽 datetime.now() 媛믪쓣 諛깆뾽?쇰줈 ?쒖슜?댁빞 ?쒕떎:\n" +
		"```python\n" +
		"import requests\n" +
		"from datetime import datetime\n\n" +
		"def fetch_time_from_web(timezone=\"Asia/Seoul\"):\n" +
		"    url = f\"http://worldtimeapi.org/api/timezone/{timezone}\"\n" +
		"    try:\n" +
		"        response = requests.get(url, timeout=3)\n" +
		"        data = response.json()\n" +
		"        return {\"datetime\": data['datetime'], \"timezone\": data['timezone'], \"utc_offset\": data['utc_offset'], \"source\": \"web_api\"}\n" +
		"    except Exception:\n" +
		"        return {\"datetime\": datetime.now().isoformat(), \"timezone\": timezone, \"utc_offset\": \"local\", \"source\": \"local_fallback\"}\n" +
		"```\n\n" +
		"[?ㅼ떆媛??댁뒪 諛??ㅽ룷痢?寃??吏移?\n" +
		"?댁뒪, ?ㅽ룷痢?寃쎄린 寃곌낵, 理쒖떊 ?ш굔 ???ㅼ떆媛????뺣낫 寃?됱씠 ?꾩슂??吏?쒕? 諛쏆븯??寃쎌슦, ?뚯씠??肄붾뱶 ?댁뿉??諛섎뱶???꾨옒??search_web ?⑥닔瑜??묒꽦?섍퀬 ?ㅽ뻾?섏뿬 ?ㅼ떆媛??곗씠?곕? ?뺣낫?댁빞 ?쒕떎. ???⑥닔??ddgs ?쇱씠釉뚮윭由?pip install ddgs)瑜??ъ슜?섎ŉ ?ㅼ젣濡?寃??寃곌낵瑜?諛섑솚?쒕떎:\n" +
		"```python\n" +
		"import subprocess, sys, warnings\n" +
		"warnings.filterwarnings('ignore', category=RuntimeWarning)\n\n" +
		"def ensure_ddgs():\n" +
		"    try:\n" +
		"        from ddgs import DDGS\n" +
		"        return DDGS\n" +
		"    except ImportError:\n" +
		"        subprocess.check_call([sys.executable, \"-m\", \"pip\", \"install\", \"ddgs\", \"-q\"])\n" +
		"        from ddgs import DDGS\n" +
		"        return DDGS\n\n" +
		"def search_web(query, max_results=5):\n" +
		"    \"\"\"ddgs(DuckDuckGo Search) ?쇱씠釉뚮윭由щ? ?듯빐 ?ㅼ떆媛?寃??寃곌낵 諛섑솚. duckduckgo_search???덈? ?ъ슜?섏? ?딅뒗??\"\"\"\n" +
		"    DDGS = ensure_ddgs()\n" +
		"    try:\n" +
		"        with warnings.catch_warnings():\n" +
		"            warnings.simplefilter('ignore')\n" +
		"            results = list(DDGS().text(query, max_results=max_results))\n" +
		"        output = []\n" +
		"        for r in results:\n" +
		"            title = r.get('title', '')\n" +
		"            body = r.get('body', '')\n" +
		"            href = r.get('href', '')\n" +
		"            output.append(f\"[{title}]\\n{body}\\n{href}\")\n" +
		"        return output if output else [\"寃??寃곌낵 ?놁쓬\"]\n" +
		"    except Exception as e:\n" +
		"        return [f\"Search error: {str(e)}\"]\n" +
		"```\n\n" +
		"[?꾧뎄 ?먮룞 ?ㅼ튂 諛??섏〈??愿由?\n" +
		"?뱀젙 ?몄뼱???쇱씠釉뚮윭由щ굹 媛쒕컻 ?섍꼍 ?꾧뎄 ?먯껜媛 ?쒖뒪?쒖뿉 遺?ы븷 寃쎌슦, ?ъ슜?먯쓽 吏???놁씠???꾨줈洹몃옩 鍮뚮뱶 ?먮뒗 ?⑦궎吏 留ㅻ땲?(Python pip, Node npm, Windows winget ??瑜??듯븳 ?먮룞 ?고????섏〈???ㅼ튂 ?ㅽ겕由쏀듃瑜??ы븿?섎뒗 肄붾뱶瑜??ㅽ뻾?섏뿬 ?닿껐?섎룄濡?泥섎━?????덈떎.\n\n" +
		"[Java ?뚯뒪肄붾뱶 ?묒꽦 吏移?\n" +
		"?먮컮(Java) 肄붾뱶瑜?援ъ꽦???뚮뒗 ?꾩떆 ?뚯씪紐낃낵??而댄뙆?쇰윭 異⑸룎???쇳븯湲??꾪빐, 吏꾩엯 ?대옒???좎뼵 ???덈?濡?'public' 吏?쒖뼱瑜?異붽??섏? 留먯븘???쒕떎. (?? 'public class Main' ???'class Main'?쇰줈留?援ъ꽦)\n\n" +
		"[?곗씠??異쒕젰 ?뺤떇 吏??\n" +
		"1. 紐⑤뱺 ?곗씠??諛?由ы룷??異쒕젰? 援ъ“?붾맂 媛濡???table) ?뺤떇, ?뺣룉??JSON, ?먮뒗 key-value ?띿쓣 ?댁슜??以꾨컮轅?援ъ“濡??щ㎎?낇븯???쒖? 異쒕젰(print)?댁빞 ?쒕떎.\n" +
		"2. 留덊겕?ㅼ슫(markdown) ?쒓렇瑜??ъ슜?섏? 留먭퀬, ?쇰컲 ?띿뒪??plain text)? ?뱀닔臾몄옄(|, -, : ??, 以꾨컮轅? ?ㅼ뿬?곌린 ?깆쓽 ?⑥닚???щ㎎留뚯쓣 ?ъ슜?섏뿬 媛?낆꽦??洹밸??뷀빐???쒕떎. ?뱁엳 ?쒕? 洹몃젮?????뚮뒗 markdown?뺤떇???꾨땶 ?띿뒪??臾몄옄濡?袁몃ŉ吏??쒕? 洹몃젮???쒕떎.\n\n" +
		"?듬????뚮뒗 '?낅땲??, '?⑸땲?? ?깆쓣 ?앸왂?섍퀬 ?쒖닠?닿? ?녿뒗 紐낆궗濡?臾몄옣??留덈Т由ы븯嫄곕굹, '?대떎', '?쒕떎' ?깆쓽 媛꾨왂???쒗쁽???ъ슜?댁빞 ?쒕떎."
}

func CallGemini(model string, history []Content) (string, error) {
	nowTime := time.Now().Format("2006-01-02 15:04:05 (MST)")

	reqBody := GenerateRequest{
		Contents: ManageHistory(history),
		SystemInstruction: &SystemInstruction{
			Parts: []Part{{Text: buildSystemPrompt(nowTime)}},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", getApiURL(model), bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status: %s, body: %s", resp.Status, string(bodyBytes))
	}

	var resObj GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&resObj); err != nil {
		return "", err
	}

	if len(resObj.Candidates) > 0 && len(resObj.Candidates[0].Content.Parts) > 0 {
		return resObj.Candidates[0].Content.Parts[0].Text, nil
	}

	return "", fmt.Errorf("empty response from Gemini")
}

func ExtractCodeBlock(response string) (string, string) {
	// 留덊겕?ㅼ슫 肄붾뱶 釉붾줉 ?⑦꽩 留ㅼ묶 ```lang ... ```
	startToken := "```"
	startIndex := strings.Index(response, startToken)
	if startIndex == -1 {
		return "", ""
	}

	// ?몄뼱 援щ텇???앸퀎???꾪븳 ?뚯떛
	rem := response[startIndex+len(startToken):]
	lineBreak := strings.Index(rem, "\n")
	if lineBreak == -1 {
		return "", ""
	}

	lang := strings.TrimSpace(rem[:lineBreak])
	// lowercase ?쒖???諛?二쇱꽍 臾몄옄 ?쒓굅
	lang = strings.ToLower(lang)

	contentStart := startIndex + len(startToken) + lineBreak + 1
	endIndex := strings.Index(response[contentStart:], "```")
	if endIndex == -1 {
		return "", ""
	}

	code := strings.TrimSpace(response[contentStart : contentStart+endIndex])
	return lang, code
}

func ExtractPowerShellCode(response string) string {
	lang, code := ExtractCodeBlock(response)
	if lang == "powershell" || lang == "pwsh" || lang == "ps1" {
		return code
	}
	return ""
}

func ExtractPythonCode(response string) string {
	lang, code := ExtractCodeBlock(response)
	if lang == "python" || lang == "py" {
		return code
	}
	return ""
}

func ExtractGoCode(response string) string {
	lang, code := ExtractCodeBlock(response)
	if lang == "go" {
		return code
	}
	return ""
}

func SaveSession(history []Content) error {
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("session.json", data, 0644)
}

func LoadSession() ([]Content, error) {
	data, err := os.ReadFile("session.json")
	if err != nil {
		return nil, err
	}
	var history []Content
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}
	return history, nil
}

// ensurePsutil checks if psutil is installed and installs it if missing
func ensurePsutil() error {
	cmd := exec.Command("python", "-c", "import psutil")
	if err := cmd.Run(); err != nil {
		installCmd := exec.Command("python", "-m", "pip", "install", "psutil", "-q")
		return installCmd.Run()
	}
	return nil
}

// runSystemInfo executes the Python script and returns its output
func runSystemInfo() (string, error) {
	if err := ensurePsutil(); err != nil {
		return "", err
	}
	cmd := exec.Command("python", "system_info.py")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// init checks for "info" argument and prints system info if present
func init() {
	if len(os.Args) > 1 && os.Args[1] == "info" {
		info, err := runSystemInfo()
		if err != nil {
			log.Fatalf("Failed to get system info: %v", err)
		}
		fmt.Print(info)
		os.Exit(0)
	}
}
