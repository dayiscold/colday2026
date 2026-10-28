# Терминальное оформление Colday

Публичная страница реализована в `frontend/` на React + TypeScript. Чёрный фон, зелёный текст,
прямоугольные рамки и индексное меню отсылают к Linux-терминалу. Цвета обложек сохраняются;
короткие помехи включаются при наведении и клавиатурном фокусе. Настройка FX сохраняется локально,
а системная настройка уменьшения движения отключает эффекты.

Портрет находится справа от центрального заголовка на широких экранах и над заголовком при ширине
до 1024 px. Меню на телефонах остаётся видимым целиком. Целевые размеры проверки: 320×740,
390×844, 768×1024, 1440×900 и 2560×1440. Скриншоты Playwright используют тестовые релизы;
эти релизы и обложки не включены в сайт и рабочее хранилище.

## Портрет

- Файл: `frontend/assets/colday-pixels.png`.
- Источник: предоставленная пользователем фотография; оригинал не изменён.
- Обработка: встроенный инструмент `imagegen`, режим редактирования, прозрачный фон.
- Фактический размер: 1050×1498, PNG RGBA с прозрачностью.
- Изображение статическое: зелёные квадратные пиксели и дизеринг, без генерации при открытии сайта.

Использованный промпт:

> Use case: style-transfer, identity-preserve. Edit the supplied portrait photograph into a production website portrait asset made entirely of crisp square green pixels, for a black Linux terminal themed musician website. Preserve this EXACT person's recognizable facial features, expression, face proportions, hairstyle, black baseball cap with flower, dark jacket, pose, and original bust framing. Do not invent a different person. Convert the photograph into detailed green monochrome pixel art, a limited palette of luminous phosphor greens from near black green to bright #78ff86, visible square pixel clusters and fine ordered dithering. Not ASCII letters, not vector contours, not an illustration with smooth shading. Keep the face clearly readable and recognizable, retain fine pixelated details on cap and jacket. Deliberate 1990s terminal bitmap / photographic dither style, no glow blur. Subject isolated with genuinely transparent alpha background; no rectangular black background, no border, no text, no UI, no extra objects, no watermark. The output will be displayed at 220–440px wide; make pixel clusters readable at these sizes. Maintain the full provided head and torso without cropping the cap.

## Шрифт

JetBrains Mono Regular и SemiBold загружаются локально в WOFF2, поддерживают кириллицу.
Источник: https://github.com/JetBrains/JetBrainsMono/tree/master/fonts/webfonts.
Лицензия SIL Open Font License 1.1 сохранена в `frontend/public/JetBrainsMono-OFL.txt`
и копируется в production-сборку вместе со шрифтами.

## Предыдущие материалы

Фотографии готического зала и их описание в `docs/design-assets.md` сохранены как материалы
предыдущего оформления. Новая страница их не использует. Существующий `static/favicon.svg`
сохранён; React-страница использует собственную терминальную иконку из `frontend/favicon.svg`.
