import { loadTemplate } from "./template-loader.js";

const headerTemplate = await loadTemplate("/static/js/shared/navigation-header-template.html");
const navTemplate = await loadTemplate("/static/js/shared/navigation-nav-template.html");

export function createNavigation(activePage) {
  return {
    template: navTemplate,
    data() {
      return {
        activePage,
      };
    },
  };
}

export function createAppHeader(activePage) {
  return {
    template: headerTemplate,
    components: {
      "app-nav": createNavigation(activePage),
    },
  };
}
