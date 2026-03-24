import { defineStore } from "pinia";

export const useAppSidebarMenuStore = defineStore("appSidebarMenu", () => {
	return [{
		'text': 'Administration',
		'is_header': true,
		'accessGroup': ['admin'],
	},{
		'icon': 'fa fa-server',
		'text': 'Serveurs',
		'accessGroup': ['admin'],
		'children': [{
			'url': '/admin/server/setup',
			'icon': 'fa-solid fa-clipboard-list',
			'text': 'Configurer',
			'accessGroup': ['admin'],
		},{
			'url': '/admin/servers',
			'icon': 'fa fa-display',
			'text': 'Counter-Strike 2',
			'accessGroup': ['admin'],
		}],
	},{
		'icon': 'fa fa-shuffle',
		'text': 'Matchmaking',
		'accessGroup': ['admin'],
		'children': [{
			'url': '/admin/matchmaking/players',
			'icon': 'fa fa-users',
			'text': 'Joueurs',
			'accessGroup': ['admin'],
		},{
			'url': '/admin/matchmaking/teams',
			'icon': 'fa-solid fa-people-group',
			'text': 'Equipes',
			'accessGroup': ['admin'],
		},{
			'url': '/admin/matchmaking/tournaments',
			'icon': 'fa fa-trophy',
			'text': 'Tournois',
			'accessGroup': ['admin'],
		}],
	},{
		'text': 'Tournoi',
		'is_header': true,
	},{
		'url': '/tournaments',
		'icon': 'fa fa-sitemap',
		'text': 'Bracket',
	},{
		'url': '/tournaments/matches',
		'icon': 'fa fa-calendar',
		'text': 'Rencontres',
	},{
		'text': 'Match Making',
		'is_header': true,
	},{
		'url': '/matchmaking',
		'icon': 'fa fa-magnifying-glass',
		'text': 'Rechercher',
		'accessGroup': ['player'],
	}];
});
