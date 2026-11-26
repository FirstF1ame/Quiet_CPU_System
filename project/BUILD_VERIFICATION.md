# Windows 실행 파일 검증

2026년 11월 26일(KST)에 [커밋 `ab71b26`](https://github.com/FirstF1ame/Quiet_CPU_System/tree/ab71b269ae41b566ab1a5b10c62a74204020e36c)의 소스와 배포 파일을 확인했습니다.

## SHA-256과 재빌드 결과

| 실행 파일 | SHA-256 | 재빌드 비교 |
| --- | --- | --- |
| [Quiet_Session_Windows_x64.exe](../Quiet_Session_Windows_x64.exe) | `cef3c453da3c252522d42ca093d23fa118cc0cb6c4c0e83d440520aff29b3986` | 바이트 단위 일치 |
| [Quiet_Session_Windows_ARM64.exe](../Quiet_Session_Windows_ARM64.exe) | `5a9073eceb081bf7e7b8acd1a1cd09a7f20e3a99dd9ba5323a9a6172d6efade3` | 바이트 단위 일치 |
| [Quiet_Session_Windows_x86.exe](../Quiet_Session_Windows_x86.exe) | `7b92a62699b5a1f35b045e35df011ebed0891c300bf7850b1a95502f6788e560` | 바이트 단위 일치 |

Go 1.25.5에서 README의 빌드 옵션을 사용했습니다. Windows 대상 아키텍처 `amd64`, `arm64`, `386`으로 각각 빌드한 결과가 해당 커밋에 포함된 EXE와 바이트 단위로 일치했습니다. `go test ./...`도 통과했습니다.

다운로드한 EXE의 SHA-256은 PowerShell에서 확인할 수 있습니다. 예시는 x64 파일입니다.

```powershell
Get-FileHash .\Quiet_Session_Windows_x64.exe -Algorithm SHA256
```

## VirusTotal 검사 결과

[사용자가 제공한 VirusTotal 화면](docs/virustotal-detection.png)은 `Quiet_CPU_System-main.zip`을 검사한 결과입니다. 첨부된 ZIP에서 다음 정보를 확인했습니다.

- ZIP SHA-256: `00afa66dc4e85e3db8ed474fecaa9d34a1360811793639d0ffd15647da658bfe`
- ZIP 크기: 3,195,279바이트
- ZIP에 기록된 GitHub 커밋: [`1d4a354`](https://github.com/FirstF1ame/Quiet_CPU_System/tree/1d4a35471facf6f64f9143e89e3de745226c8d3d)
- ZIP 내부 14개 파일의 Git 블롭 해시와 크기가 해당 커밋과 모두 일치합니다. 내부 EXE 3종의 SHA-256은 위 표와 일치합니다.

화면의 대표 위협 라벨은 `trojan.generic`이며, 다음 탐지 3건이 표시됩니다.

| 탐지 엔진 | 표시 결과 |
| --- | --- |
| Bkav Pro | `W32.Malware.C293072F` |
| Elastic | `Malicious (moderate Confidence)` |
| McAfee Scanner | `Trojan:Win/GenericY.HVE` |

Microsoft, Bitdefender, Kaspersky 등은 `Undetected`로 표시됩니다. Arctic Wolf는 파일 형식을 처리하지 못했다고 표시합니다. 제공된 화면에서는 검사일, 전체 엔진 수, 보고서의 SHA-256을 확인할 수 없습니다. 위 ZIP 해시는 첨부 파일에서 직접 계산한 값입니다.

ZIP 보고서만으로는 어떤 내부 파일이 탐지를 유발했는지, 또는 오탐인지 확인할 수 없습니다. 개별 EXE 결과는 VirusTotal의 **Relations → Bundled files**에서 파일별 SHA-256으로 확인하거나, EXE를 개별 검사해 확인할 수 있습니다.

[공식 VirusTotal 압축 파일 검사 안내](https://docs.virustotal.com/docs/compressed-files) · [탐지 결과 및 오탐 안내](https://docs.virustotal.com/docs/false-positive)

### ARM64 실행 파일 개별 검사

[Quiet_Session_Windows_ARM64.exe의 VirusTotal 보고서](https://www.virustotal.com/gui/file/5a9073eceb081bf7e7b8acd1a1cd09a7f20e3a99dd9ba5323a9a6172d6efade3/detection) 링크에 포함된 SHA-256은 위 표의 ARM64 배포본과 일치합니다.

| 엔진 | 사용자가 확인한 결과 |
| --- | --- |
| Arctic Wolf | `Unsafe` |
| Acronis (Static ML) | `Undetected` |

위 판정은 사용자가 전달한 결과입니다. 이 기록을 작성할 때 보고서 화면을 직접 열어 전체 탐지 수나 검사 시각을 확인하지 못했습니다. 이 판정은 ARM64 EXE 개별 검사에 해당합니다. x64와 x86의 개별 보고서는 이 문서에 아직 연결되어 있지 않습니다.
