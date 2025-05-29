# OpenAPI Aggregator Operator

🚧 **현재 개발 진행 중입니다** 🚧

## 프로젝트 소개
Kubernetes 클러스터 내의 서비스들의 OpenAPI 스펙을 자동으로 수집하고 통합하여 보여주는 Operator입니다.

## 주요 기능
- 라벨 셀렉터를 통한 서비스 자동 발견
- OpenAPI 스펙 실시간 수집
- Swagger UI를 통한 통합 문서 제공
- 네임스페이스 기반 필터링 지원

## 프로젝트 구조
```
.
├── api/                   # CRD API 정의
├── cmd/                   # operator 메인 엔트리포인트
├── internal/              # 컨트롤러 구현
├── pkg/                   # 재사용 가능한 패키지
│   └── swagger/          # Swagger UI 서버
└── config/               # Kubernetes 매니페스트
    ├── crd/              # CRD 정의
    ├── rbac/             # 권한 설정
    └── manager/          # operator 배포 설정
```

## 개발 환경 설정
```bash
# 필요한 도구 설치
make install-tools

# CRD 설치
make install

# operator 실행
make run
```

## 사용 예시
```yaml
apiVersion: observability.aggregator.io/v1alpha1
kind: OpenAPIAggregator
metadata:
  name: example-aggregator
spec:
  displayNamePrefix: "API-"
  labelSelector:
    app: myapp
  path: /v3/api-docs
  port: "8080"
```

## 현재 개발 상태
- [x] 기본 Operator 구조 구현
- [x] OpenAPI 스펙 수집 로직 구현
- [x] Swagger UI 통합
- [x] 실시간 스펙 조회 기능
- [ ] 인증/인가 기능 추가
- [ ] 메트릭스 수집 추가
- [ ] 고가용성 지원

## 라이선스
Apache License 2.0