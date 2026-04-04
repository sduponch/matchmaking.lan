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
		},{
			'url': '/admin/matchmaking/match-configs',
			'icon': 'fa fa-file-code',
			'text': 'Profils de match',
			'accessGroup': ['admin'],
		}],
	},{
		'icon': 'fa fa-gamepad',
		'text': 'Counter-Strike 2',
		'accessGroup': ['admin'],
		'children': [{
			'url': '/admin/cs2',
			'icon': 'fa fa-map',
			'text': 'Map pool officiel',
			'accessGroup': ['admin'],
		},{
			'url': '/admin/matchmaking/encounters',
			'icon': 'fa fa-shield-halved',
			'text': 'Rencontres',
			'accessGroup': ['admin'],
		},{
			'url': '/admin/matchmaking/tournaments',
			'icon': 'fa fa-trophy',
			'text': 'Tournois',
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
