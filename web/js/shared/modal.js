export function createModal() {
  return {
    template: `
      <div class="modal">
        <div class="modal-content">
          <div class="modal-header">
            <h3>{{ title }}</h3>
            <div v-if="hasHeaderActions" class="modal-header-actions">
              <slot name="header-actions"></slot>
            </div>
            <button @click="$emit('close')">✕</button>
          </div>
          <div class="modal-body">
            <slot></slot>
          </div>
          <div v-if="hasFooter" class="modal-footer">
            <slot name="footer"></slot>
          </div>
        </div>
      </div>
    `,
    props: {
      title: {
        type: String,
        required: true,
      },
    },
    computed: {
      hasHeaderActions() {
        return !!this.$slots["header-actions"];
      },
      hasFooter() {
        return !!this.$slots.footer;
      },
    },
  };
}
