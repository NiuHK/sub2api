.PHONY: build build-backend build-frontend test test-backend test-frontend test-frontend-critical deploy deploy-backup deploy-current deploy-down deploy-status deploy-logs

FRONTEND_CRITICAL_VITEST := \
	src/i18n/__tests__/localeKeyCompleteness.spec.ts \
	src/api/__tests__/client.spec.ts \
	src/api/__tests__/tokenRefresh.spec.ts \
	src/api/__tests__/keys.bulkUpdate.spec.ts \
	src/components/account/__tests__/OpenAIReferralCell.spec.ts \
	src/components/account/__tests__/OpenAIReferralCell.transport.spec.ts \
	src/components/account/__tests__/OpenAIQuotaResetCell.spark_shadow.spec.ts \
	src/components/keys/__tests__/BulkEditKeysModal.spec.ts \
	src/components/admin/user/__tests__/UserPlatformQuotaModal.spec.ts \
	src/views/user/__tests__/KeysView.spec.ts \
	src/api/__tests__/channelMonitorV2.spec.ts \
	src/views/auth/__tests__/LinuxDoCallbackView.spec.ts \
	src/views/auth/__tests__/WechatCallbackView.spec.ts \
	src/views/user/__tests__/PaymentView.spec.ts \
	src/views/user/__tests__/PaymentResultView.spec.ts \
	src/views/user/__tests__/ChannelStatusView.mode.spec.ts \
	src/components/user/profile/__tests__/ProfileInfoCard.spec.ts \
	src/views/admin/__tests__/SettingsView.spec.ts \
	src/features/channel-monitor-v2/__tests__/designSystem.structure.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorFormat.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorZoom.spec.ts

# 一键编译前后端
build: build-backend build-frontend

# 编译后端（复用 backend/Makefile）
build-backend:
	@$(MAKE) -C backend build

# 编译前端（需要已安装依赖）
build-frontend:
	@pnpm --dir frontend run build

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@$(MAKE) -C backend test

test-frontend:
	@pnpm --dir frontend run lint:check
	@pnpm --dir frontend run typecheck
	@$(MAKE) test-frontend-critical

test-frontend-critical:
	@pnpm --dir frontend exec vitest run $(FRONTEND_CRITICAL_VITEST)

# Build the current source and deploy it behind the host Caddy at api2.pinellia.uk.
deploy:
	@bash deploy/local-deploy.sh

# Switch only the app container; keep PostgreSQL, Redis, volumes and Caddy intact.
# The preserved image is tagged once before the first source upgrade.
deploy-backup:
	@docker image inspect sub2api-local:backup-before-group-routing >/dev/null
	@docker compose --env-file deploy/.env -f deploy/docker-compose.yml -f deploy/docker-compose.source.yml -f deploy/docker-compose.backup.yml up -d --no-deps --no-build --force-recreate --wait --wait-timeout 180 sub2api

deploy-current:
	@docker image inspect sub2api-local:latest >/dev/null
	@docker compose --env-file deploy/.env -f deploy/docker-compose.yml -f deploy/docker-compose.source.yml up -d --no-deps --no-build --force-recreate --wait --wait-timeout 180 sub2api

deploy-down:
	@docker compose --env-file deploy/.env -f deploy/docker-compose.yml -f deploy/docker-compose.source.yml down

deploy-status:
	@docker compose --env-file deploy/.env -f deploy/docker-compose.yml -f deploy/docker-compose.source.yml ps

deploy-logs:
	@docker compose --env-file deploy/.env -f deploy/docker-compose.yml -f deploy/docker-compose.source.yml logs -f --tail=100
