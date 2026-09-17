<template>
  <el-popover placement="left" :trigger="trigger" :width="width">
    <template #reference>
      <el-button type="primary" circle :size="size">
        <SvgIcon :name="icon" />
      </el-button>
    </template>
    <div class="table-action-menu">
      <slot />
    </div>
  </el-popover>
</template>

<script setup lang="ts">
/**
 * 表格操作按钮组件 - TableAction
 *
 * 统一的操作按钮组件，用于表格行内操作
 * 使用 el-popover 弹出操作菜单（零业务依赖通用组件）
 *
 * 使用示例：
 * <TableAction>
 *   <el-button text type="primary" @click="handleEdit(scope.row)">
 *     <SvgIcon name="elementEdit" /> 修改
 *   </el-button>
 *   <el-button text type="danger" @click="handleDelete(scope.row)">
 *     <SvgIcon name="elementDelete" /> 删除
 *   </el-button>
 * </TableAction>
 */

interface Props {
  // 触发方式：click / hover / focus / contextmenu
  trigger?: 'click' | 'hover' | 'focus' | 'contextmenu'
  // 图标名称，默认 elementStar（收藏图标）
  icon?: string
  // 按钮大小：large / default / small
  size?: 'large' | 'default' | 'small'
  // 弹出框宽度
  width?: number | string
}

withDefaults(defineProps<Props>(), {
  trigger: 'click',
  icon: 'elementStar',
  size: 'default',
  width: 'auto'
})
</script>

<style scoped>
.table-action-menu {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.table-action-menu :deep(.el-button) {
  justify-content: flex-start;
  padding: 8px 12px;
  margin: 0;
  width: 100%;
}

.table-action-menu :deep(.el-button + .el-button) {
  margin-left: 0;
}
</style>
