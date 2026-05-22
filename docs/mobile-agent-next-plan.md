# cc-base Mobile Agent Next Plan

## Summary

鏈鍒掔敤浜庝慨澶嶅綋鍓嶅井淇?QQ 绉诲姩绔?agent 浜や簰鐨勫墿浣欓棶棰樸€傜洰鏍囦笉鏄噸鍐欐暣涓郴缁燂紝鑰屾槸鎶婂凡缁忛獙璇佸彲鐢ㄧ殑 Go `cc-controller` 璺緞琛ラ綈锛氬懡浠ゅ彲鍙戠幇銆佸洖璋冨洖鍒版纭笭閬撱€佽緭鍑烘棤姹℃煋銆侀厤缃彲浠?GitHub 妯℃澘鎭㈠銆?
褰撳墠宸查獙璇佽兘鍔涳細

- `/cc` 鏄?session-aware锛屽璇濅笂涓嬫枃鍙欢缁€?- `/鐘舵€乣 鍙煡鐪嬮」鐩€乻ession銆佺瓑寰呴槦鍒椼€佹椿鍔ㄤ换鍔″拰鏈€杩?run銆?- 鎵ц鍨嬩换鍔′細鍏堢敓鎴愮‘璁ゅ崱锛宍/鎵ц RunId` 鎴栧敮涓€绛夊緟浠诲姟鏃跺洖澶?`濂?ok/鍙互/纭` 鍙墽琛屻€?- `CC_EXECUTE_WORK_DIR` 娌欑洅鎵ц宸查獙璇侊紝鍙妸鏂囦欢鍒涘缓鍒?`C:\path\to\cc-base\test`銆?- `/椤圭洰`銆乣/鍒囬」鐩甡 宸叉敮鎸佸椤圭洰宸ヤ綔鐩綍銆?
褰撳墠鍓╀綑闂锛?
- ~~`/鏌ョ湅`銆乣/鍕樺療` 涓嶆槸 cc-connect command~~ 鈫?**宸蹭慨澶?2026-05-19**锛氭柊澧?`[[commands]]`
- ~~Codex 鍥炵瓟閲屾贩鍏?`taskkill` 鎴愬姛淇℃伅~~ 鈫?**宸蹭慨澶?2026-05-19**锛歝odex.go 杩囨护 + 鍗曟祴
- ~~cc-base skill 妯℃澘涓庢湰鏈洪厤缃紓绉粇~ 鈫?**宸蹭慨澶?2026-05-19**锛氬弽鍚戝悓姝?+ 鑴辨晱
- QQ Codex 鍥炶皟鍙兘鍥為敊娓犻亾锛坄--reply-project` 闅旂鏈疄鐜帮級
- CC_MODEL 榛樿鍊兼湭鍦?controller 灞傞潰浼犻€?鈫?**宸蹭慨澶?2026-05-19**锛歝c.go + start.ps1

## Scope

鍙鐞嗙Щ鍔ㄧ浜や簰鍙潬鎬э紝涓嶅仛浠ヤ笅浜嬫儏锛?
- 涓嶉噸鍐?cc-connect銆?- 涓嶆敼 Claude/Codex 妯″瀷璋冪敤绛栫暐銆?- 涓嶅疄鐜板畬鏁村骞冲彴娑堟伅鎬荤嚎銆?- 涓嶆妸鏅€氳嚜鐒惰瑷€ fallback 鍏ㄩ儴鎺ョ鍒?Go router銆?
## Step 1: 鎭㈠ `/鏌ョ湅` 鍜屽父鐢ㄧ姸鎬佸埆鍚?鉁?宸插畬鎴?
### 闂

cc-connect v1.3.2 鐨?`[[aliases]]` 涓嶅尮閰?`/` 鍓嶇紑娑堟伅銆俙/鏌ョ湅` 濡傛灉鍙啓 alias锛屼細琚綋鎴愭湭鐭ュ懡浠よ浆缁?Agent銆?
### 淇敼

鍦ㄤ互涓嬮厤缃腑鏂板鐪熷疄 `[[commands]]`锛?
- `C:\path\to\cc-base\cc-connect\config.toml`
- `%USERPROFILE%\.cc-connect\config.toml`
- `C:\path\to\cc-base\scripts\config.toml.template`

鏂板鍛戒护锛?
```toml
[[commands]]
name = "鏌ョ湅"
description = "鏌ョ湅褰撳墠绯荤粺鐘舵€併€侀」鐩€佺瓑寰呴槦鍒楀拰鏈€杩戜换鍔?
exec = "E:\\ai\\selfwork_ytl\\controller\\cc-controller.exe status"

[[commands]]
name = "鍕樺療"
description = "鏌ョ湅褰撳墠绯荤粺鐘舵€併€侀」鐩€佺瓑寰呴槦鍒楀拰鏈€杩戜换鍔?
exec = "E:\\ai\\selfwork_ytl\\controller\\cc-controller.exe status"

[[commands]]
name = "鐪嬬湅"
description = "鏌ョ湅褰撳墠绯荤粺鐘舵€併€侀」鐩€佺瓑寰呴槦鍒楀拰鏈€杩戜换鍔?
exec = "E:\\ai\\selfwork_ytl\\controller\\cc-controller.exe status"
```

妯℃澘涓娇鐢ㄥ崰浣嶈矾寰勶細

```toml
exec = "YOUR_PROJECT_ROOT\\controller\\cc-controller.exe status"
```

淇濈暀 `/鐘舵€乣锛屾妸 `/鏌ョ湅`銆乣/鍕樺療`銆乣/鐪嬬湅` 閮戒綔涓虹湡瀹?command锛屼笉渚濊禆 alias銆?
### 楠岃瘉

寰俊鍜?QQ 鍚勫彂閫侊細

```text
/鏌ョ湅
/鍕樺療
/鐪嬬湅
/鐘舵€?```

棰勬湡锛?
- 閮借繑鍥炲悓涓€绫荤郴缁熺姸鎬併€?- 涓嶅啀鍑虹幇 `涓嶆槸 cc-connect 鍛戒护`銆?- 涓嶅啀瑙﹀彂 `Invalid signature in thinking block`銆?
## Step 2: 淇 QQ Codex 閫氶亾 鈥?閮ㄥ垎瀹屾垚

### 闂

褰撳墠 config 涓?Codex/QQ project 浠嶅瓨鍦紝浣嗙敤鎴锋姤鍛娾€滃ぇ鍙?QQ 缁欏皬鍙?QQ Codex 娑堟伅涓嶈鈥濄€傚彲鑳藉師鍥犳湁涓夌被锛?
1. QQ 鍙戦€佽€呬笉鍖归厤 `allow_from`銆?2. `鍙戠粰codex` 宸蹭粠鏃?direct codex project 鏀规垚 `/闂甤odex`锛屼絾 QQ 骞冲彴娌℃湁姝ｇ‘鍔犺浇鏂?command銆?3. `cc-controller` 鍥炶皟鍐欐鍙戝埌 `cc` project锛屽鑷?QQ 瑙﹀彂鐨?Codex 鏈€缁堢粨鏋滃洖鍒板井淇℃垨鍏朵粬閫氶亾銆?
### 淇敼

鍏堜笉鏀瑰ぇ鏋舵瀯锛屾寜鏈€灏忚矾寰勪慨澶嶏細

1. 纭 QQ project 浠嶉厤缃細
   - platform/provider 鏄?QQ/OneBot銆?   - `allow_from` 鍖呭惈澶у彿 QQ銆?   - `[[aliases]]` 涓?Codex/GPT alias 鍙槧灏勫埌 `/闂甤odex`銆?2. 纭 `[[commands]] name = "闂甤odex"` 鍦ㄩ儴缃?config 涓瓨鍦紝涓?exec 鎸囧悜锛?
```text
C:\path\to\cc-base\controller\cc-controller.exe ask-codex {{args}}
```

3. 缁?`ask-codex` 澧炲姞 source-aware callback 鍙傛暟锛岄伩鍏嶇‖缂栫爜鍥?`cc`锛?
```text
cc-controller.exe ask-codex --reply-project codex {{args}}
```

濡傛灉 cc-connect 鏃犳硶浼犲叆鏉ユ簮 project锛屽垯鍏堝湪 QQ/codex 椤圭洰鐨?command 閲屽浐瀹?`--reply-project codex`锛屽井淇?cc 椤圭洰鐨?command 鍥哄畾 `--reply-project cc`銆?
### 楠岃瘉

浠?QQ 澶у彿鍙戠粰 QQ 灏忓彿锛?
```text
/鐘舵€?鍙戠粰codex 2+2绛変簬鍑?```

棰勬湡锛?
- `/鐘舵€乣 鍦?QQ 杩斿洖銆?- Codex 鍚姩娑堟伅鍦?QQ 杩斿洖銆?- Codex 鏈€缁堢瓟妗堜篃鍦?QQ 杩斿洖銆?- 寰俊涓嶆敹鍒拌繖娆?QQ 瑙﹀彂鐨?Codex 鍥炶皟銆?
## Step 3: 娓呯悊 Codex 杈撳嚭閲岀殑 taskkill 鍣煶鍜屼贡鐮?鉁?宸插畬鎴?
### 闂

Codex 鍥炵瓟涓嚭鐜扮被浼煎唴瀹癸細

```text
锟缴癸拷: 锟斤拷锟斤拷止 PID ...
SUCCESS: The process with PID ...
```

杩欐槸 cleanup/taskkill 鐨?stdout/stderr 娣峰叆浜嗘ā鍨嬬瓟妗堛€傚畠涓嶆槸 Codex 鍐呭锛屼篃涓嶅簲璇ヨ繘鍏ョ敤鎴峰洖璋冦€?
### 淇敼

鍦?Go controller 涓鐞嗕袱灞傦細

1. 鎵ц `taskkill` 鏃朵涪寮?stdout/stderr锛?
```go
cmd.Stdout = io.Discard
cmd.Stderr = io.Discard
```

2. 鍦?Codex 杈撳嚭娓呯悊鍑芥暟閲屽鍔犲厹搴曡繃婊わ細

- `SUCCESS: The process with PID`
- `鎴愬姛: 宸茬粓姝?PID`
- mojibake 鍓嶇紑 `锟缴癸拷:`
- 鍙寘鍚?PID 缁堟淇℃伅鐨勮

涓嶈杩囨护鐪熷疄 Codex answer銆?
### 楠岃瘉

杩愯锛?
```text
鍙戠粰codex 浣犳槸浠€涔堟ā鍨?```

棰勬湡锛?
- 鍥炵瓟鍙寘鍚?Codex 姝ｆ枃鍜屽缓璁笅涓€姝ャ€?- 涓嶅嚭鐜?taskkill PID 琛屻€?- 涓嶅嚭鐜颁腑鏂?taskkill 涔辩爜銆?
鏂板 Go 鍗曟祴锛?
- 杈撳叆鍖呭惈鑻辨枃 taskkill 琛?+ 姝ｆ枃锛岃緭鍑哄彧淇濈暀姝ｆ枃銆?- 杈撳叆鍖呭惈涓枃 mojibake taskkill 琛?+ 姝ｆ枃锛岃緭鍑哄彧淇濈暀姝ｆ枃銆?
## Step 4: 閰嶇疆鍚屾鍜屾ā鏉垮浐鍖?鉁?宸插畬鎴?
### 闂

cc-base 浠?GitHub 鍚屾鍥炴潵鍚庯紝妯℃澘鍙兘缂哄皯鏈満鏂板鍛戒护锛屽鑷撮噸瑁呮垨鍚屾鍚庡姛鑳芥秷澶便€?
### 淇敼

鎶婃湰鏈哄凡楠岃瘉閰嶇疆鍚屾鍥炴ā鏉匡紝浣嗘ā鏉垮繀椤昏劚鏁忥細

- 淇濈暀 command 鍚嶇О鍜?exec 缁撴瀯銆?- 璺緞浣跨敤 `YOUR_PROJECT_ROOT`銆?- QQ/寰俊 token銆乤ccount_id銆佺湡瀹?sender ID 涓嶈繘鍏ユā鏉裤€?- `scripts/config.toml.template` 蹇呴』鍖呭惈锛?  - `/cc`
  - `/鐘舵€乣
  - `/鏌ョ湅`
  - `/鍕樺療`
  - `/鐪嬬湅`
  - `/闂甤odex`
  - `/codex缁撴灉`
  - `/鎵ц`
  - `/鍙栨秷`
  - `/椤圭洰`
  - `/鍒囬」鐩甡
  - `ok/濂?鍙互/纭` alias 鍒?`/鎵ц`
  - Codex/GPT alias 鍒?`/闂甤odex`
  - CC/Opus alias 鍒?`/cc`

### 楠岃瘉

鍦?cc-base 浠撳簱鎵弿锛?
```powershell
Select-String -Path .\**\* -Pattern "ilinkai|wx_token|account_id|D:\\research-work|C:\\cc-base|:7890|:7891"
```

棰勬湡锛?
- 涓嶅嚭鐜扮湡瀹?token銆佺湡瀹?account_id銆佺湡瀹炴湰鏈烘晱鎰熻矾寰勩€?- 妯℃澘鍙嚭鐜板崰浣嶇銆?
## Step 5: 鏂囨。鏇存柊 鈥?閮ㄥ垎瀹屾垚

### 淇敼

鏇存柊浠ヤ笅鏂囦欢锛?
- `SKILL.md`
- `README.md`
- `docs/qq-setup.md`
- `docs/config-management.md`

蹇呴』鍐欐竻妤氾細

- `/鏌ョ湅` 鏄湡瀹?command锛屼笉鏄?alias銆?- `/鐘舵€乣 鍜?`/鏌ョ湅` 绛変环銆?- QQ Codex 鍥炶皟蹇呴』鍥炲埌 QQ project锛屼笉搴旂‖缂栫爜鍒板井淇?`cc` project銆?- `鍙戠粰cc/闂甤c/opus` 璧?session-aware `/cc`銆?- `鍙戠粰codex/闂甤odex/gpt` 璧?`/闂甤odex`銆?- 鎵ц浠诲姟鐨勭煭纭锛歚ok/濂?鍙互/纭` 鍙湪鍞竴 waiting 浠诲姟鏃惰嚜鍔ㄦ墽琛岋紱澶氫釜 waiting 鏃跺繀椤?`/鎵ц 1` 鎴?`/鎵ц RunId`銆?
## Step 6: 鏈€缁堥獙鏀舵竻鍗?
### 寰俊绔?
```text
/鏌ョ湅
/cc 鎴戝彨鏉庡洓锛岃浣?/cc 鎴戝彨浠€涔堬紵
/cc 鍒涘缓鏂囦欢 mobile-ok.txt
濂?```

棰勬湡锛?
- `/鏌ョ湅` 姝ｅ父杩斿洖鐘舵€併€?- `/cc` 鑳借浣忓悓涓€ session 鍐呬笂涓嬫枃銆?- 鎵ц纭鍗℃樉绀虹湡瀹?`CC_EXECUTE_WORK_DIR`銆?- `濂絗 鑳芥墽琛屽敮涓€绛夊緟浠诲姟銆?
### QQ Codex 绔?
```text
/鐘舵€?鍙戠粰codex 浣犳槸浠€涔堟ā鍨?```

棰勬湡锛?
- QQ 鑳芥敹鍒板惎鍔ㄦ秷鎭€?- QQ 鑳芥敹鍒版渶缁?Codex 绛旀銆?- 绛旀鏃?taskkill 鍣煶銆?- 寰俊涓嶄覆鍙版敹鍒?QQ 鐨?Codex 绛旀銆?
### 閰嶇疆鎭㈠娴嬭瘯

浠?`scripts/config.toml.template` 閲嶆柊鐢熸垚涓€浠芥祴璇?config锛屾浛鎹㈠崰浣嶇鍚庡簲鍏峰鍚岀瓑鍛戒护闆嗗悎銆?
## Step 7: 澶氭ā鍨?QQ 璺敱锛圖eepSeek / GLM锛?
### 鑳屾櫙

config.toml 宸查厤缃笁涓?provider锛圤penAI 宸插～ key锛孌eepSeek/GLM 鐣欑┖寰呭～锛夈€傜敤鎴峰笇鏈涗粠 QQ 鍒嗗埆闂笉鍚屾ā鍨嬨€?
### 鏂规鍊欓€?
1. **姣忎釜妯″瀷涓€涓?cc-connect project**锛歈Q 閲岄€氳繃 `/闂甦eepseek`銆乣/闂甮lm` 鍛戒护璺敱鍒扮嫭绔?project
2. **璧?cc-controller 鍛戒护璺敱**锛氬姞 `[[commands]]` 鍒嗗埆璋冧笉鍚屾ā鍨嬬殑 CLI锛堢被浼煎井淇?`/闂甤odex`锛?
### 鍓嶇疆鏉′欢

- DeepSeek API key
- GLM (鏅鸿氨) API key
- 纭畾鏂规鍚庡疄鐜拌矾鐢?
### 鐘舵€?
寰呭疄鏂斤紝API key 鍒颁綅鍚庡紑濮嬨€?
## Step 8: CC_MODEL 鏀寔 鉁?宸插畬鎴?
### 淇敼锛?026-05-19锛?
- `cc.go`锛氳鍙?`CC_MODEL` 鐜鍙橀噺锛屼紶 `--model` 缁?Claude CLI
- `start.ps1`锛氶粯璁よ缃?`CC_MODEL=claude-opus-4-6`锛屼笉褰卞搷 CLI 鍏ㄥ眬榛樿
- `docs/env-vars.md`锛氬凡鏇存柊鏂囨。

## 韪╁潙璁板綍锛?026-05-19 QQ 鎺ュ叆锛?
| 鍧?| 鐜拌薄 | 鏍瑰洜 | 淇 |
|----|------|------|------|
| NapCat WS 绔彛 | cc-connect `ws connect failed` | NapCat WebSocket Server 榛樿绔彛 6099 璺?WebUI 鍐茬獊 | 鏀逛负 3001 |
| NapCat 娑堟伅鏍煎紡 | 杩炰笂绔嬪埢鏂紑 `close 1005` | 娑堟伅鏍煎紡璁句负 Array锛宑c-connect 涓嶆敮鎸?| 鏀逛负 String |
| NapCat 鍚敤寮€鍏?| WebSocket 涓嶇洃鍚?| WebSocket Server 閰嶇疆閲?鍚敤"鏈墦寮€ | 鎵撳紑鍚敤寮€鍏?|
| 鍚姩鏃跺簭 | cc-connect 姣?NapCat 鍏堝氨缁?| `docker start` 鍚?NapCat 闇€ 1-3 鍒嗛挓鍚姩 | 閲嶅惎 cc-connect 鎴?start.ps1 鍔?sleep |
| API key 鏍煎紡 | `${sk-proj-...}` 琚В鏋愪负鐜鍙橀噺 | 鐢ㄦ埛鎶?key 鏀捐繘浜?`${}` 鍗犱綅绗﹂噷 | 鐩存帴鍐?key锛屼笉鐢?`${}` |
| Docker 闀滃儚鍚?| `napneko-docker:latest` 鎷夊彇 403 | 鏈湴闀滃儚鏄?`napcat-docker`锛屼笉鏄?`napneko-docker` | 鐢ㄦ湰鍦板凡鏈夐暅鍍忓悕 |

## 椋庨櫓鍜岃竟鐣?
- 濡傛灉 QQ OneBot/NapCat 鏈韩鏂繛锛屾湰璁″垝鍙兘璁╅厤缃纭紝涓嶈兘鏇夸唬 QQ 缃戝叧鎺掗殰銆?- 濡傛灉 cc-connect 涓嶆彁渚?source project 缁?command锛岀煭鏈熷彧鑳介€氳繃涓嶅悓 project 鐨?command 鍐欐 `--reply-project`銆?- `/鏌ョ湅` 淇蹇呴』鍐欒繘鐪熷疄 `[[commands]]`锛屽彧鍐?alias 鏃犳晥銆?- taskkill 杩囨护鍙兘杩囨护杩涚▼娓呯悊鍣煶锛屼笉鑳芥帺鐩栫湡瀹?Codex 閿欒銆?
## 鍓╀綑宸ヤ綔

1. **Step 2**锛歈Q Codex `--reply-project` 鍥炶皟闅旂锛堥槻姝?QQ 缁撴灉涓插埌寰俊锛?2. **Step 5**锛歋KILL.md銆乧onfig-management.md 鏂囨。鏇存柊
3. **Step 6**锛氬井淇?+ QQ 鍙岀鏈€缁堥獙鏀?4. **Step 7**锛氬妯″瀷璺敱锛圖eepSeek/GLM锛夛紝寰?API key 鍒颁綅

宸插畬鎴愶細Step 1 鉁?Step 3 鉁?Step 4 鉁?Step 8 鉁?

