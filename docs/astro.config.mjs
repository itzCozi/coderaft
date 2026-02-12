
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightThemeRapide from 'starlight-theme-rapide'
import sitemap from '@astrojs/sitemap'


export default defineConfig({
	site: 'https://coderaft.ar0.eu',
	integrations: [
    sitemap(),
		starlight({
			title: 'coderaft',
			description: 'Isolated development environments using Docker containers',
			favicon: '/favicon.png',
			logo: {
				replacesTitle: true,
        light: './src/assets/logo-dark.png',
        dark: './src/assets/logo.png',
      },
      lastUpdated: true,
			components: {
				Footer: './src/components/CustomFooter.astro',
			},
			head: [
				{
					tag: 'style',
					content: `
					  /* Hide Starlight search button/modal */
					  site-search { display: none !important; }
					`
				},
			],
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/itzcozi/coderaft' },
				{ icon: 'telegram', label: 'Telegram', href: 'http://t.me/coderaftcli' }
			],
			editLink: {
        baseUrl: 'https://github.com/itzcozi/coderaft/edit/main/docs/',
      },
			sidebar: [
				{
					label: 'Getting Started',
					items: [
						{ label: 'Introduction', slug: 'docs/intro' },
						{ label: 'Installation', slug: 'docs/install' },
						{ label: 'Quick Start', slug: 'docs/start' },
						{ label: 'FAQ', slug: 'docs/faq' },
					],
				},
				{
					label: 'Configuration',
					collapsed: true,
					items: [
						{ label: 'Configuration Files', slug: 'docs/configuration' },
						{ label: 'Templates & Setup', slug: 'docs/templates' },
					],
				},
				{
					label: 'Maintenance',
					collapsed: true,
					items: [
						{ label: 'Cleanup & Maintenance', slug: 'docs/maintenance' },
						{ label: 'Troubleshooting', slug: 'docs/troubleshooting' },
					],
				},
				{
					label: 'Reference',
					items: [
						{ label: 'CLI Commands', slug: 'docs/cli' },
					],
				},
			],
			plugins: [starlightThemeRapide()],
		}),
	],
});
