# Release Notes v3.1.0

## EN

icicle 3.1.0 is the final stabilization pass for the current generation: one desktop runtime (Wails), synchronized versioning, performance upgrades, and release-ready packaging.

### Highlights
- Single desktop GUI runtime path (legacy internal GUI removed)
- Heavy table performance pack: virtualized rendering + chunk paint + debounce + cache
- Fast scan cache for `tree`, `heavy`, WizMap
- Adaptive log polling and lower idle overhead
- Queue execution optimization (batch grouping + dedup)
- Path normalization and safer quick actions
- Persistent UI/scan settings
- Scanner hot-path optimizations (`internal/scan`)
- Updated docs, roadmap, release assets

## RU

icicle 3.1.0 — финальная стабилизация текущего поколения: единый desktop runtime (Wails), синхронизация версии, ускорения и подготовка к релизной поставке.

### Основное
- Единый путь GUI (legacy `internal/gui` удалён)
- Пакет ускорений heavy-таблицы: виртуализация + поэтапный рендер + debounce + cache
- Кэш быстрых сканов `tree`, `heavy`, WizMap
- Адаптивный polling логов и снижение нагрузки в idle
- Оптимизация очередей (группировка + дедуп путей)
- Нормализация путей и безопасные быстрые действия
- Сохранение параметров UI/сканирования
- Оптимизации hot-path в `internal/scan`
- Обновлённые docs/roadmap/релизные ассеты
