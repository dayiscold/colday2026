import { useState } from 'react';
import { useMotion, useReleases } from './hooks';
import type { Release } from './types';
import portrait from '../assets/colday-pixels.png';

const TELEGRAM = 'https://t.me/coldaybigstepper';
const EMAIL = 'deadplugbooking' + '@' + 'gmail.com';
const DATE_RU = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' });

function formatDate(value: string) {
  const datePart = /^(\d{4}-\d{2}-\d{2})/.exec(value);
  if (!datePart) return value;
  const date = new Date(datePart[1] + 'T12:00:00Z');
  return Number.isNaN(date.getTime()) ? value : DATE_RU.format(date);
}

function Navigation() {
  return <nav className="navigation" aria-label="Основная навигация">
    <a className="brand" href="#top" aria-label="COLDAY — главная"><span aria-hidden="true">&gt;_</span> COLDAY</a>
    <div className="nav-sections">
      <a href="#top"><span>[0]</span> home</a>
      <a href="#music"><span>[1]</span> music</a>
      <a href="#contacts"><span>[2]</span> contacts</a>
    </div>
    <a className="telegram" href={TELEGRAM} target="_blank" rel="noopener noreferrer" aria-label="COLDAY в Telegram"><span aria-hidden="true">↗</span> telegram</a>
  </nav>;
}

function PixelPortrait() {
  return <figure className="portrait">
    <div className="portrait-caption" aria-hidden="true"><span>./colday.bmp</span><span>RGB: 0 / G / 0</span></div>
    <div className="portrait-image"><img src={portrait} alt="Портрет COLDAY из зелёных пикселей" width="1050" height="1498" fetchPriority="high" /></div>
    <figcaption><span>СВЕТ СРЕДИ ХОЛОДА</span><span aria-hidden="true">■ ■ ■</span></figcaption>
  </figure>;
}

function Hero({ featured }: { featured?: Release }) {
  const label = featured?.title
    ? `Новинка — «${featured.title}»${featured.release_type ? ' ' + featured.release_type : ''}`
    : 'Новые релизы';
  return <section className="hero" id="top" aria-labelledby="hero-title">
    <div className="hero-session" aria-hidden="true"><span><i className="status-dot" /> colday@world:~</span><span>SESSION / 001</span></div>
    <div className="hero-stage">
      <div className="hero-copy">
        <p className="command" aria-hidden="true"><span>$</span> ./colday<span className="cursor">_</span></p>
        <h1 id="hero-title" translate="no">COLDAY</h1>
        <p className="tagline">Свет Среди Холода</p>
        <a href="#music" className="button hero-cta"><span className="status-dot" aria-hidden="true" /><span>{label}</span><span aria-hidden="true">↙</span></a>
      </div>
      <PixelPortrait />
    </div>
    <div className="hero-bottom" aria-hidden="true"><span>Прямое соединение.</span><span className="scroll-hint">scroll to explore <span>↓</span></span><span>EST. COLDAY<br />[ SOUND ON YOUR TERMS ]</span></div>
  </section>;
}

function SectionHeading({ number, path, title }: { number: string; path: string; title: string }) {
  return <header className="section-heading">
    <div><p className="command" aria-hidden="true"><span>$</span> {path}</p><h2>{title}<span className="heading-dot" aria-hidden="true">_</span></h2></div>
    <span className="section-number" aria-hidden="true">/{number}</span>
  </header>;
}

function Cover({ src }: { src: string }) {
  const [failed, setFailed] = useState(false);
  return <div className="cover">
    {src && !failed ? <img src={src} alt="" width="600" height="600" loading="lazy" decoding="async" onError={() => setFailed(true)} />
      : <div className="cover-placeholder" role="img" aria-label="Обложка отсутствует"><span aria-hidden="true">[ Ø ]</span><small>NO COVER</small></div>}
    <span className="cover-noise" aria-hidden="true" />
    <span className="cover-corner" aria-hidden="true">+</span>
  </div>;
}

function ListenLink({ release }: { release: Release }) {
  if (!release.bandlink?.trim()) return null;
  return <a className="button listen" href={release.bandlink.trim()} target="_blank" rel="noopener noreferrer" aria-label={`Слушать «${release.title}»`}><span aria-hidden="true">▶</span> Слушать <span aria-hidden="true">↗</span></a>;
}

function ReleaseCard({ release, featured = false, index }: { release: Release; featured?: boolean; index: number }) {
  const meta = [release.release_type, featured ? formatDate(release.original_date) : /^\d{4}/.exec(release.original_date)?.[0]].filter(Boolean).join(' / ');
  return <article className={featured ? 'release featured-release' : 'release catalog-release'} aria-label={release.title}>
    <Cover key={release.cover} src={release.cover} />
    <div className="release-info">
      <p className="release-index" aria-hidden="true">{featured ? '[ LATEST RELEASE ]' : `[ ${String(index).padStart(2, '0')} ]`}</p>
      <p className="release-meta">{meta}</p>
      <h3>{release.title}</h3>
      <ListenLink release={release} />
    </div>
  </article>;
}

function Music({ releases, loading, error }: ReturnType<typeof useReleases>) {
  const featured = releases.find(release => release.featured) ?? releases[0];
  const rest = releases.filter(release => release.id !== featured?.id);
  const message = error ? 'Не удалось загрузить релизы. Обновите страницу, чтобы попробовать ещё раз.'
    : loading ? 'Загрузка релизов…' : releases.length === 0 ? 'Новые релизы скоро появятся здесь.' : '';
  return <section className="section music" id="music" aria-label="Релизы">
    <SectionHeading number="01" path="ls ./music" title="Релизы" />
    <div className="catalog-bar" aria-hidden="true"><span>DISCOGRAPHY / AUDIO FILES</span><span>{String(releases.length).padStart(2, '0')} RECORDS</span></div>
    {message && <p className={`music-status${error ? ' is-error' : ''}`} role="status"><span aria-hidden="true">{error ? '[ ! ]' : '[ … ]'}</span> {message}</p>}
    {featured && <ReleaseCard release={featured} featured index={1} />}
    {rest.length > 0 && <div className="release-grid">{rest.map((release, index) => <ReleaseCard key={release.id} release={release} index={index + 2} />)}</div>}
    <div className="section-end" aria-hidden="true"><span>END OF DIRECTORY</span><span>└── ~/music</span></div>
  </section>;
}

function Contacts() {
  return <section className="section contacts" id="contacts" aria-label="Контакты">
    <SectionHeading number="02" path="cat ./contacts" title="На связи" />
    <div className="contact-body"><p className="contact-label">БУКИНГ / СОТРУДНИЧЕСТВО</p><a className="mail" href={`mailto:${EMAIL}`}>{EMAIL}<span aria-hidden="true">↗</span></a><p className="contact-note"><span aria-hidden="true">&gt;</span> Начнём с сообщения.</p></div>
    <footer><span>Свет Среди Холода<span className="cursor" aria-hidden="true">_</span></span><span>© {new Date().getFullYear()} COLDAY</span></footer>
  </section>;
}

function MotionToggle({ paused, reduced, toggle }: ReturnType<typeof useMotion>) {
  const label = paused ? 'Включить эффекты' : 'Приостановить эффекты';
  return <button className="motion-toggle" type="button" aria-label={label} title={label} aria-pressed={paused} onClick={toggle} hidden={reduced}><span aria-hidden="true">{paused ? '▶' : 'Ⅱ'}</span><span className="motion-label" aria-hidden="true">FX</span></button>;
}

export function App() {
  const music = useReleases();
  const motion = useMotion();
  const featured = music.releases.find(release => release.featured) ?? music.releases[0];
  return <div className="site" data-motion={motion.disabled ? 'paused' : 'running'}>
    <a className="skip-link" href="#main">Перейти к содержимому</a>
    <Navigation />
    <main id="main" tabIndex={-1}><Hero featured={featured} /><Music {...music} /><Contacts /></main>
    <MotionToggle {...motion} />
  </div>;
}
