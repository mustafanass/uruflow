import {
  BaseEdge,
  EdgeText,
  getSmoothStepPath,
  type EdgeProps,
} from '@xyflow/react'

function ResourceEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  markerStart,
  markerEnd,
  style,
  label,
  labelStyle,
  labelBgStyle,
  data,
  interactionWidth,
}: EdgeProps) {
  const [path, labelX, labelY] = getSmoothStepPath({
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
  })
  return (
    <>
      <BaseEdge
        id={id}
        path={path}
        markerStart={markerStart}
        markerEnd={markerEnd}
        style={style}
        interactionWidth={interactionWidth}
      />
      {data?.motion === true && (
        <path
          d={path}
          pathLength={100}
          className="graph-edge-motion"
          aria-hidden="true"
        />
      )}
      {label && (
        <EdgeText
          x={labelX}
          y={labelY}
          label={label}
          labelStyle={labelStyle}
          labelBgStyle={labelBgStyle}
          labelBgPadding={[5, 3]}
          labelBgBorderRadius={3}
        />
      )}
    </>
  )
}

export const edgeTypes = { resource: ResourceEdge }
