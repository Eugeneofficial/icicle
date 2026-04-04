package commands

import "time"

const (
	// GlaciarClassThreshold — путь считается "glacier-class heavy" при объёме >= 500 GB.
	GlaciarClassThreshold int64 = 500 * 1024 * 1024 * 1024

	// BlackIceThreshold — файл считается "black-ice payload" при размере >= 4 GB.
	BlackIceThreshold int64 = 4 * 1024 * 1024 * 1024

	// CooldownDuration — минимальный интервал между обработкой одного и того же файла в watch mode.
	CooldownDuration = 2 * time.Second

	// CooldownCleanupAge — записи старше этого срока удаляются из cooldown-карты.
	CooldownCleanupAge = 15 * time.Second

	// CooldownMaxEntries — максимальное количество записей в cooldown-карте до очистки.
	CooldownMaxEntries = 4096

	// RetryDelay — задержка между попытками перемещения файла при ошибке занятости.
	RetryDelay = 180 * time.Millisecond

	// StableFileCheckDelay — задержка между проверками стабильности файла.
	StableFileCheckDelay = 200 * time.Millisecond

	// StableFileSettleTime — файл считается стабильным, если не менялся дольше этого времени.
	StableFileSettleTime = 700 * time.Millisecond
)
