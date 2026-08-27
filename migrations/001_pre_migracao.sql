-- ==========================================================
-- 001 · PREPARA UM BANCO QUE JÁ TEM DADOS
-- ==========================================================
-- Rode ESTE arquivo ANTES de subir a API com AUTO_MIGRATE=true, se o banco já
-- tiver dados de verdade.
--
-- Por quê: as correções de segurança criaram índices UNIQUE e mudaram o tipo de
-- duas colunas. Num banco vazio o AutoMigrate faz tudo sozinho. Num banco COM
-- dados, se já existir duplicata, o índice UNIQUE não é criado, o AutoMigrate
-- aborta e a API NÃO SOBE.
--
-- Este arquivo limpa as duplicatas (mantendo sempre o registro mais antigo) e
-- converte os tipos, tudo dentro de uma transação: ou passa inteiro, ou não
-- muda nada.
--
-- Como rodar:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/001_pre_migracao.sql
--
-- ⚠️ FAÇA BACKUP ANTES:
--   pg_dump "$DATABASE_URL" > backup_antes_da_migracao.sql

BEGIN;

-- ----------------------------------------------------------
-- 1. study_sessions: user_id e space_id nasceram como TEXT
-- ----------------------------------------------------------
-- Sem o tipo uuid, qualquer JOIN com users/spaces falhava ("operator does not
-- exist: text = uuid") e os índices ficavam grandes e lentos. É a tabela mais
-- consultada do sistema.
DO $$
BEGIN
	IF EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_name = 'study_sessions' AND column_name = 'user_id' AND data_type = 'text'
	) THEN
		-- Linhas com id inválido não têm como ser convertidas: são lixo de teste.
		DELETE FROM study_sessions
		WHERE user_id !~ '^[0-9a-fA-F-]{36}$' OR space_id !~ '^[0-9a-fA-F-]{36}$';

		ALTER TABLE study_sessions ALTER COLUMN user_id  TYPE uuid USING user_id::uuid;
		ALTER TABLE study_sessions ALTER COLUMN space_id TYPE uuid USING space_id::uuid;
		RAISE NOTICE 'study_sessions: user_id/space_id convertidos de text para uuid';
	ELSE
		RAISE NOTICE 'study_sessions: colunas já estão em uuid, nada a fazer';
	END IF;
END $$;

-- ----------------------------------------------------------
-- 2. Remove duplicatas antes dos índices UNIQUE
-- ----------------------------------------------------------
-- Regra em todas: fica o registro MAIS ANTIGO (menor ctid), somem os demais.

-- 2.1 space_permissions — um usuário só pode ter uma permissão por turma
DELETE FROM space_permissions a USING space_permissions b
WHERE a.ctid > b.ctid AND a.space_id = b.space_id AND a.user_id = b.user_id;

-- 2.2 mission_completions — impedia XP em dobro na missão relâmpago
DELETE FROM mission_completions a USING mission_completions b
WHERE a.ctid > b.ctid AND a.mission_id = b.mission_id AND a.user_id = b.user_id;

-- 2.3 certificates — um certificado por aluno por turma
DELETE FROM certificates a USING certificates b
WHERE a.ctid > b.ctid AND a.space_id = b.space_id AND a.user_id = b.user_id;

-- 2.4 attendance_records — impedia +10 XP repetido na mesma chamada
DELETE FROM attendance_records a USING attendance_records b
WHERE a.ctid > b.ctid AND a.session_id = b.session_id AND a.student_id = b.student_id;

-- 2.5 user_badges — mesmo emblema entregue duas vezes
DELETE FROM user_badges a USING user_badges b
WHERE a.ctid > b.ctid AND a.user_id = b.user_id AND a.badge_id = b.badge_id;

-- 2.6 notification_reads — marcação de "lida" duplicada
DELETE FROM notification_reads a USING notification_reads b
WHERE a.ctid > b.ctid AND a.notification_id = b.notification_id AND a.user_id = b.user_id;

-- 2.7 followers — seguir a mesma pessoa duas vezes
DELETE FROM followers a USING followers b
WHERE a.ctid > b.ctid AND a.follower_id = b.follower_id AND a.following_id = b.following_id;

-- ----------------------------------------------------------
-- 3. reviews: colunas novas da repetição espaçada
-- ----------------------------------------------------------
-- A revisão passou a ter dono, degrau na escada de intervalos e data de
-- conclusão. As linhas antigas recebem o dono a partir do autor da página
-- (a melhor informação disponível) e começam no primeiro degrau.
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS user_id      uuid;
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS stage        integer DEFAULT 0;
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS completed_at timestamptz;

UPDATE reviews r
SET user_id = p.created_by_id
FROM pages p
WHERE r.note_id = p.id AND r.user_id IS NULL;

-- Revisão órfã (página apagada) não tem como ser atribuída a ninguém.
DELETE FROM reviews WHERE user_id IS NULL;

-- ----------------------------------------------------------
-- 4. verification_codes: contador de tentativas (anti força-bruta)
-- ----------------------------------------------------------
ALTER TABLE verification_codes ADD COLUMN IF NOT EXISTS attempts integer DEFAULT 0;

-- ----------------------------------------------------------
-- 5. Higiene: registros expirados que nunca eram limpos
-- ----------------------------------------------------------
DELETE FROM verification_codes WHERE expires_at < NOW();
DELETE FROM password_resets    WHERE expires_at < NOW();

COMMIT;

-- ==========================================================
-- Depois deste arquivo, suba a API com AUTO_MIGRATE=true uma vez para o GORM
-- criar os índices e as colunas restantes. Confira no log:
--   ✅ AutoMigrate concluído com sucesso!
-- Em seguida pode voltar para AUTO_MIGRATE=false (a API sobe mais rápido).
-- ==========================================================
