import { expect, it } from 'vitest';
import { resolveCameraPose, type CameraSnapshot } from './camera';

const rotatedAndZoomed: CameraSnapshot = {
  position: [17, 8, -12], target: [1, -2, 3], reset: 0, view: 'perspective',
};

it('preserves the user pose when labels, graph bounds, or viewport aspect change', () => {
  for (const fitDistance of [28, 74, 130]) {
    expect(resolveCameraPose(rotatedAndZoomed, { reset: 0, view: 'perspective' }, fitDistance)).toEqual({
      position: [17, 8, -12], target: [1, -2, 3],
    });
  }
});

it('refits after an explicit reset even when the selected camera view is unchanged', () => {
  const pose = resolveCameraPose(rotatedAndZoomed, { reset: 1, view: 'perspective' }, 40);
  expect(Math.hypot(...pose.position)).toBeCloseTo(40);
  expect(pose.position[2]).toBeGreaterThan(0);
  expect(pose.target).toEqual([0, 0, 0]);
});

it('applies a newly selected view and then retains subsequent user adjustments', () => {
  const front = resolveCameraPose(rotatedAndZoomed, { reset: 0, view: 'front' }, 40);
  expect(front).toEqual({ position: [0, 0, 40], target: [0, 0, 0] });
  const adjusted: CameraSnapshot = { position: [4, 6, 19], target: [0, 0, 0], reset: 0, view: 'front' };
  expect(resolveCameraPose(adjusted, { reset: 0, view: 'front' }, 90).position).toEqual([4, 6, 19]);
});

it('fits the first scene without a saved pose', () => {
  expect(resolveCameraPose(null, { reset: 0, view: 'front' }, 28)).toEqual({ position: [0, 0, 28], target: [0, 0, 0] });
});
