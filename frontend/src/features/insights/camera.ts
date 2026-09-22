export type CameraView = 'perspective' | 'front' | 'side' | 'above';
type Coordinates = [number, number, number];
type CameraPose = { position: Coordinates; target: Coordinates };
type CameraSelection = { reset: number; view: CameraView };
export type CameraSnapshot = CameraPose & CameraSelection;

/** Data, locale and viewport changes retain the pose; only explicit camera actions refit. */
export function resolveCameraPose(saved: CameraSnapshot | null, selection: CameraSelection, fitDistance: number): CameraPose {
  if (saved && saved.reset === selection.reset && saved.view === selection.view) {
    return { position: [...saved.position], target: [...saved.target] };
  }
  const { view } = selection;
  const direction: Coordinates = view === 'front' ? [0, 0, 1] : view === 'side' ? [.8, .1, .65] : view === 'above' ? [.1, .85, .65] : [.22, .16, 1];
  const scale = fitDistance / Math.hypot(...direction);
  return { position: [direction[0] * scale, direction[1] * scale, direction[2] * scale], target: [0, 0, 0] };
}
