<script lang="ts">
  import { deleteConfirmDialog } from '../../lib/stores/ui';
  import { deleteNodes } from '../../lib/stores/config';
  import MessageDialog from '../gtk/MessageDialog.svelte';

  async function handleConfirm() {
    const ids = $deleteConfirmDialog.nodeIds;
    deleteConfirmDialog.set({ open: false, nodeIds: [], count: 0 });
    if (ids.length > 0) {
      await deleteNodes(ids);
    }
  }

  function handleCancel() {
    deleteConfirmDialog.set({ open: false, nodeIds: [], count: 0 });
  }

  let message = $derived(
    $deleteConfirmDialog.count === 1
      ? 'Delete selected node and ALL of its contents?'
      : `Delete ${$deleteConfirmDialog.count} node(s) and ALL of their contents?`
  );
</script>

<MessageDialog
  open={$deleteConfirmDialog.open}
  type="warning"
  title="Confirm Delete"
  primaryText={message}
  buttons="yes_no"
  onresponse={(resp) => {
    if (resp === 'yes') {
      handleConfirm();
    } else {
      handleCancel();
    }
  }}
/>
