// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

const repo = 'https://github.com/lawale/quorum';

export default defineConfig({
	site: 'https://quorum.olawalelawal.dev',
	trailingSlash: 'always',
	redirects: { '/docs/': '/docs/getting-started/quickstart/' },
	integrations: [
		starlight({
			title: 'Quorum',
			description:
				'Quorum is an open-source, self-hosted approval engine. Maker-checker, four-eyes and multi-stage approvals as one service for every application, with policies as data, layered guards, a full audit trail, webhooks and embeddable widgets.',
			logo: { src: './src/assets/logo.svg', alt: 'Quorum' },
			favicon: '/favicon.svg',
			social: [{ icon: 'github', label: 'GitHub', href: repo }],
			editLink: { baseUrl: `${repo}/edit/master/site/` },
			customCss: ['./src/styles/custom.css'],
			components: { Hero: './src/components/Hero.astro' },
			head: [
				{ tag: 'meta', attrs: { property: 'og:image', content: 'https://quorum.olawalelawal.dev/og.png' } },
				{ tag: 'meta', attrs: { name: 'twitter:card', content: 'summary_large_image' } },
			],
			sidebar: [
				{
					label: 'Getting started',
					items: [
						{ label: 'Quick start', slug: 'docs/getting-started/quickstart' },
						{ label: 'Console and widgets locally', slug: 'docs/getting-started/full-stack' },
						{ label: 'Docker', slug: 'docs/getting-started/docker' },
						{ label: 'Walkthrough: a wire transfer', slug: 'docs/getting-started/walkthrough' },
					],
				},
				{
					label: 'Concepts',
					items: [
						{ label: 'How Quorum works', slug: 'docs/concepts/how-it-works' },
						{ label: 'Permission guards', slug: 'docs/concepts/permission-guards' },
						{ label: 'Bypass rules', slug: 'docs/concepts/bypass-rules' },
						{ label: 'Reviewer scoping', slug: 'docs/concepts/reviewer-scoping' },
						{ label: 'Display templates', slug: 'docs/concepts/display-templates' },
					],
				},
				{
					label: 'Integrate',
					items: [
						{ label: 'REST API', slug: 'docs/integrate/api' },
						{ label: 'Go client', slug: 'docs/integrate/go-client' },
						{ label: 'Embeddable widgets', slug: 'docs/integrate/widgets' },
						{ label: 'Webhooks', slug: 'docs/integrate/webhooks' },
					],
				},
				{
					label: 'Operate',
					items: [
						{ label: 'Configuration', slug: 'docs/operate/configuration' },
						{ label: 'Authentication modes', slug: 'docs/operate/authentication' },
						{ label: 'Databases', slug: 'docs/operate/databases' },
						{ label: 'Admin console', slug: 'docs/operate/console' },
						{ label: 'Health and metrics', slug: 'docs/operate/health' },
					],
				},
				{ label: 'Demo and sample apps', slug: 'docs/demo' },
			],
		}),
	],
});
