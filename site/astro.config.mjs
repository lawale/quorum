// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

const repo = 'https://github.com/lawale/quorum';

export default defineConfig({
	site: 'https://quorum.olawalelawal.dev',
	trailingSlash: 'always',
	redirects: {
		'/docs/': '/docs/getting-started/quickstart/',
		'/docs/integrate/go-client/': '/docs/clients/go/',
		'/docs/integrate/typescript-client/': '/docs/clients/typescript/',
		'/docs/integrate/dotnet-client/': '/docs/clients/dotnet/',
	},
	integrations: [
		starlight({
			title: 'Quorum',
			description:
				'Quorum is an open-source, self-hosted approval engine. Maker-checker, four-eyes and multi-stage approvals as one service for every application, with policies as data, layered guards, a full audit trail, webhooks and embeddable widgets.',
			logo: { light: './src/assets/logo-light.svg', dark: './src/assets/logo-dark.svg', alt: 'Quorum' },
			favicon: '/favicon.svg',
			social: [{ icon: 'github', label: 'GitHub', href: repo }],
			editLink: { baseUrl: `${repo}/edit/master/site/` },
			customCss: ['./src/styles/custom.css'],
			components: { Hero: './src/components/Hero.astro', Footer: './src/components/Footer.astro' },
			head: [
				{ tag: 'link', attrs: { rel: 'apple-touch-icon', href: '/apple-touch-icon.png' } },
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
						{ label: 'Webhooks', slug: 'docs/integrate/webhooks' },
						{ label: 'Embeddable widgets', slug: 'docs/integrate/widgets' },
					],
				},
				{
					label: 'Clients',
					items: [
						{
							label: 'Go',
							collapsed: true,
							items: [
								{ label: 'Overview', slug: 'docs/clients/go' },
								{ label: 'Identity and authentication', slug: 'docs/clients/go/authentication' },
								{ label: 'Decisions: streams, waiting and webhooks', slug: 'docs/clients/go/decisions' },
								{ label: 'Policies and webhook subscriptions', slug: 'docs/clients/go/policies' },
							],
						},
						{
							label: 'TypeScript',
							collapsed: true,
							items: [
								{ label: 'Overview', slug: 'docs/clients/typescript' },
								{ label: 'Identity and authentication', slug: 'docs/clients/typescript/authentication' },
								{ label: 'Decisions: streams, waiting and webhooks', slug: 'docs/clients/typescript/decisions' },
								{ label: 'Policies and webhook subscriptions', slug: 'docs/clients/typescript/policies' },
							],
						},
						{
							label: '.NET',
							collapsed: true,
							items: [
								{ label: 'Overview', slug: 'docs/clients/dotnet' },
								{ label: 'ASP.NET Core', slug: 'docs/clients/dotnet/aspnet-core' },
								{ label: 'MediatR', slug: 'docs/clients/dotnet/mediatr' },
								{ label: 'Identity and authentication', slug: 'docs/clients/dotnet/authentication' },
								{ label: 'Decisions: streams, waiting and webhooks', slug: 'docs/clients/dotnet/decisions' },
								{ label: 'Policies and webhook subscriptions', slug: 'docs/clients/dotnet/policies' },
							],
						},
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
