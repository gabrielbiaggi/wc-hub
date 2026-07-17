<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
echarts.use([LineChart, GridComponent, TooltipComponent, CanvasRenderer])
const props = defineProps<{ values: number[] }>(); const root = ref<HTMLElement>(); let chart: echarts.ECharts | undefined
const render = () => chart?.setOption({ animationDuration: 500, grid:{left:0,right:0,top:12,bottom:0}, tooltip:{trigger:'axis',backgroundColor:'#0b111d',borderColor:'#26364e',textStyle:{color:'#e2e8f0',fontSize:11},axisPointer:{lineStyle:{color:'#49e29d55'}}}, xAxis:{type:'category',boundaryGap:false,show:false,data:props.values.map((_,i)=>i)}, yAxis:{type:'value',show:false,min:(v:{min:number})=>v.min-8,max:(v:{max:number})=>v.max+8}, series:[{type:'line',data:props.values,smooth:.35,showSymbol:false,lineStyle:{color:'#49e29d',width:2},areaStyle:{color:new echarts.graphic.LinearGradient(0,0,0,1,[{offset:0,color:'rgba(73,226,157,.28)'},{offset:1,color:'rgba(73,226,157,0)'}])}}] })
const resize = () => chart?.resize()
onMounted(()=>{ if(root.value){ chart=echarts.init(root.value);render();window.addEventListener('resize',resize)} }); watch(()=>props.values,render); onBeforeUnmount(()=>{window.removeEventListener('resize',resize);chart?.dispose()})
</script>
<template><div ref="root" class="h-full min-h-48 w-full" role="img" aria-label="Uso agregado de infraestrutura nas últimas 24 horas" /></template>

