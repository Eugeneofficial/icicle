# Release Notes v5.0.0

## EN

icicle 5.0.0 is a major desktop generation update.

Highlights:
- Full visual redesign of the Wails desktop app with a more intentional information hierarchy.
- Rebuilt `Dashboard`, `Analyze`, and `Automation` layouts for clearer operational flow.
- New frontend asset structure with extracted desktop CSS (`cmd/icicle-wails/frontend/app.css`).
- Safer watch mode: files are no longer moved immediately while they may still be changing.
- Stricter limited-scan behavior under concurrency.
- CLI consistency fixes and broader regression coverage.

This release is the first version that feels like a cohesive storage operations cockpit rather than a collection of useful panels.

## RU

icicle 5.0.0 — это большое обновление desktop-поколения.

Главное:
- Полный визуальный редизайн Wails-приложения с более сильной иерархией информации.
- Пересобраны layout-блоки `Dashboard`, `Analyze` и `Automation` под более понятный рабочий сценарий.
- Улучшена структура frontend-asset'ов: CSS вынесен в `cmd/icicle-wails/frontend/app.css`.
- Режим watch стал безопаснее: файлы больше не переносятся мгновенно, пока могут ещё изменяться.
- Более строгая работа limited-сканов при конкурентном обходе.
- Выравнивание CLI-поведения и расширение регрессионных тестов.

Это первый релиз, где приложение воспринимается как единый storage-operations cockpit, а не набор полезных экранов.
