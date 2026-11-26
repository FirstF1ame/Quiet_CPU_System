# Quiet_CPU_System

Quiet Session은 Windows 10·11 노트북에서 일정 시간 동안 CPU 성능 상한을 낮추는 앱입니다.

## 사용 이유

강의실, 도서관, 회의실처럼 조용한 곳에서 노트북 팬 소리가 갑자기 커질 수 있습니다. 문서 작성이나 웹 검색처럼 높은 CPU 성능이 필요하지 않은 작업을 할 때, Quiet Session으로 CPU 발열을 줄여 팬이 급가속할 가능성을 낮출 수 있습니다.

Windows 전원 설정을 사용하므로 다양한 제조사의 노트북에서 사용할 수 있습니다. 앱은 팬 RPM을 직접 지정하지 않으며, 노트북의 냉각 시스템이 온도에 따라 팬을 조절합니다. CPU 상한을 낮추면 고부하 작업이 느려질 수 있고, GPU 사용량이 높은 작업에서는 소음 감소 효과가 제한적일 수 있습니다.

## 내 컴퓨터에 맞는 EXE 고르기

파일은 **한 개만** 내려받으면 됩니다. 컴퓨터 모델이나 CPU 이름을 몰라도 Windows 설정에서 확인할 수 있습니다.

1. 화면 아래 **시작** 버튼을 누르고 **설정**을 엽니다.
2. **시스템 → 정보**로 이동합니다.
3. **장치 사양**의 **시스템 종류**를 보고 아래 표에서 일치하는 파일을 선택합니다.

| 시스템 종류에 보이는 문구 | 어떤 컴퓨터인가요? | 받을 파일 |
| --- | --- | --- |
| **64비트 운영 체제**와 **x64 기반 프로세서** | 64비트 Windows를 쓰는 Intel 또는 AMD PC | [Quiet_Session_Windows_x64.exe](Quiet_Session_Windows_x64.exe) |
| **64비트 운영 체제**와 **ARM 기반 프로세서** | ARM 기반 Windows PC. 예를 들어 Snapdragon X를 탑재한 일부 노트북 | [Quiet_Session_Windows_ARM64.exe](Quiet_Session_Windows_ARM64.exe) |
| **32비트 운영 체제** | 32비트 Windows 10 PC. 뒤에 **x64 기반 프로세서**라고 써 있어도 이 행을 선택합니다. | [Quiet_Session_Windows_x86.exe](Quiet_Session_Windows_x86.exe) |

**64비트**라고 표시되면 뒤에 나오는 **x64 기반** 또는 **ARM 기반** 문구까지 확인하세요.  
[Microsoft의 Windows 시스템 종류 확인 안내](https://support.microsoft.com/en-us/windows/experience/compatibility/32-bit-and-64-bit-windows-frequently-asked-questions)

## 사용법

1. 위 표에서 선택한 파일 링크를 열고 **다운로드** 버튼을 누릅니다. 다운로드가 끝나면 파일을 더블클릭합니다. Python이나 Go 설치는 필요 없습니다.
2. **사용 시간**을 1~180분 사이로 입력하고, **최대 프로세서 상태**를 60%·75%·85% 중 선택합니다. 처음에는 기본 설정인 **75%**로 짧게 시험해 볼 수 있습니다.
3. **시작**을 누릅니다. 창에서 남은 시간과 CPU·RAM·GPU 사용량을 확인합니다. 사용량은 약 1초마다 갱신되며 GPU 데이터를 가져올 수 없으면 **측정 불가**로 표시합니다.
4. 시간이 끝나거나 **지금 종료**를 누르거나 창을 닫으면 이전 전원 계획을 복구합니다. Windows 종료 등으로 복구가 중단된 경우에는 같은 계정에서 앱을 다시 실행하면 복구를 시도합니다.

## 실행 파일 검증

[SHA-256 및 재빌드 검증 기록](project/BUILD_VERIFICATION.md)에서 배포 EXE 3종의 체크섬과 VirusTotal 검사 상태를 확인할 수 있습니다.

## 소스에서 빌드

Go 1.25 이상을 사용합니다. 저장소의 `project` 폴더에서 PowerShell을 열고 아래 명령으로 x64용 EXE를 빌드합니다. ARM64는 `GOARCH`를 `arm64`로, 32비트는 `386`으로 바꿉니다.

```powershell
New-Item -ItemType Directory -Force build | Out-Null
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -buildvcs=false -trimpath -ldflags="-H=windowsgui -s -w" -o build/Quiet_Session_Windows_x64.exe .
```

모의 전원 계획과 사용량 계산 테스트는 `go test ./...`로 실행할 수 있습니다. Windows 실기기에서의 전원 정책 적용, 화면 표시, 소음 감소 효과는 아직 검증되지 않았습니다.
