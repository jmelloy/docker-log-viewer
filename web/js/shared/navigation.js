import { loadTemplate } from "./template-loader.js";

const headerTemplate = await loadTemplate("/templates/navigation-header.html");
const navTemplate = await loadTemplate("/templates/navigation-nav.html");

export function createNavigation(activePage) {
  return {
    template: navTemplate,
    data() {
      return {
        activePage
      };
    }
  };
}

export function createAppHeader(activePage) {
  return {
    template: headerTemplate,
    components: {
      'app-nav': createNavigation(activePage)
    }
  };
}
