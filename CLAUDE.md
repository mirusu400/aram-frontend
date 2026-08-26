# CLAUDE.md, aram-frontend 코딩 규칙

CODEX.md(제품 계약)와 docs/를 함께 따른다. 이 문서는 **코드 구조 규칙**을 정의한다.

## 패키지 구조

- `cmd/aram-frontend`, 데스크톱 진입점만. 로직 금지.
- `mobile`, gomobile 바인딩만. 로직 금지.
- `frontend`, 나머지 전부. **단일 flat 패키지를 유지한다.**
  하위 패키지 신설 금지(사유: Shell 메서드가 파일 15곳에 분산된 현 구조에서 패키지
  분리는 alias-shim 없이 불가능. 도입은 별도 승인 필요).

## frontend 내부 계층 (파일 그룹)

같은 패키지지만 파일은 4계층으로 나뉜다. 파일명 접두사로 소속을 드러낸다.

**L0 도메인/데이터**, backend.go, state.go, commands.go, picker.go, settings*.go,
localization.go, version.go, input_bindings.go, strutil.go
**L1 서비스/위젯 (Shell 비의존)**, issue_relay.go, update_client.go, audio_output.go,
design_system.go, ime_text_input.go, icon.go, debug_bundle.go, artifact_io.go,
플랫폼 스텁(url_\*, folder_\*, locale_\*, window_\*, picker_\*)
**L2 Shell (앱 상태·흐름)**, shell.go, shell_\*.go, panels.go, pacing.go,
binding_capture.go, welcome.go
**L3 UI (표시)**, shell_ui.go, ui_\*.go, render.go, touch.go, focus_mode.go,
virtual_keypad.go, panel_text.go

## 의존 방향 규칙 (무엇이 무엇을 참조할 수 있는가)

```
L3 UI ──► L2 Shell ──► L1 서비스 ──► L0 도메인
   └────────┴─────────────────────────► L0
```

1. **L0**: L0끼리만 참조. `Shell`/`shellUI` 심볼 참조 금지. ebitenui import 금지.
   (예외: input_bindings.go의 `ebiten.Key` 등 입력 타입은 허용.)
2. **L1**: L0만 참조. `Shell`/`shellUI` 참조 금지. ebitenui import는
   design_system.go·ime_text_input.go만 허용.
3. **L2**: L0·L1 참조 가능. L3 소유는 shell.go의 `interfaceUI` 필드와 그 호출로 한정.
   `Shell` 메서드는 L2 파일에만 정의한다 (render 계열 `draw*` 메서드는 L3 예외).
4. **L3**: L2의 상태를 읽고 Shell 메서드를 핸들러로 호출. L1 직접 호출 금지
   (design_system·ime 위젯 제외). 네트워크/파일 I/O 금지. `settings.save()` 직접
   호출 금지, 모든 변경은 Shell 메서드를 거친다.
5. **플랫폼 스텁**: 빌드 태그로 갈라지는 소형 함수만. `Shell` 참조 금지.
   런타임 `GOOS` 분기 대신 빌드 태그 (CODEX.md 계승).
6. 테스트 파일은 계층 제약 없음.

## 파일 크기 상한

- 비테스트 소스: **600라인**. 테스트: **900라인**.
- 초과가 불가피하면 사유와 함께 승인을 받는다.

## 검증

```powershell
gofmt -l .        # 변경 없음이어야 함
go build ./...
go test ./...
go vet ./...
```
