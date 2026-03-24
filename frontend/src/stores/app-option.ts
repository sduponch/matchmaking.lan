import { defineStore } from "pinia";

export const useAppOptionStore = defineStore("appOption", () => {
	return {
		appMode: 'dark',
		appRtlMode: false,
		appThemeClass: '',
		appCoverClass: '',
		appBoxedLayout: false,
		appHeaderHide: false,
		appHeaderSearchToggled: false,
		appSidebarToggled: true,
		appSidebarCollapsed: false,
		appSidebarMobileToggled: false,
		appSidebarMobileClosed: false,
		appSidebarHide: false,
		appContentFullHeight: false,
		appContentClass: '',
		appTopNav: false,
		appFooter: false,
		appFooterFixed: false,
		appThemePanelToggled: false,
	}
});
