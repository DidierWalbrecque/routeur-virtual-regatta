import { useEffect, type RefObject } from "react";

const LONG_PRESS_MS = 450;
const TIP_VISIBLE_MS = 1800;

/**
 * Sur écran tactile, les infobulles au survol n'apparaissent jamais. Un appui
 * long sur un bouton du conteneur affiche son infobulle (classe `show-tip`)
 * sans déclencher le bouton ; un appui court garde son action habituelle.
 */
export function useLongPressTips(container: RefObject<HTMLElement | null>, selector: string) {
  useEffect(() => {
    const root = container.current;
    if (!root) return;
    let pressTimer = 0;
    let hideTimer = 0;
    let shown: HTMLElement | null = null;
    let suppressClick = false;

    const hide = () => {
      shown?.classList.remove("show-tip");
      shown = null;
    };
    const onPointerDown = (event: PointerEvent) => {
      if (event.pointerType !== "touch") return;
      const button = (event.target as HTMLElement | null)?.closest<HTMLElement>(selector);
      if (!button) return;
      suppressClick = false;
      window.clearTimeout(pressTimer);
      pressTimer = window.setTimeout(() => {
        hide();
        button.classList.add("show-tip");
        shown = button;
        suppressClick = true;
        window.clearTimeout(hideTimer);
        hideTimer = window.setTimeout(hide, TIP_VISIBLE_MS);
      }, LONG_PRESS_MS);
    };
    const cancelPress = () => window.clearTimeout(pressTimer);
    // Le clic qui suit un appui long n'active pas le bouton.
    const onClick = (event: MouseEvent) => {
      if (!suppressClick) return;
      suppressClick = false;
      event.preventDefault();
      event.stopPropagation();
    };
    const onContextMenu = (event: Event) => {
      if ((event.target as HTMLElement | null)?.closest(selector)) event.preventDefault();
    };

    root.addEventListener("pointerdown", onPointerDown);
    root.addEventListener("pointerup", cancelPress);
    root.addEventListener("pointercancel", cancelPress);
    root.addEventListener("pointerleave", cancelPress);
    root.addEventListener("click", onClick, true);
    root.addEventListener("contextmenu", onContextMenu);
    return () => {
      window.clearTimeout(pressTimer);
      window.clearTimeout(hideTimer);
      hide();
      root.removeEventListener("pointerdown", onPointerDown);
      root.removeEventListener("pointerup", cancelPress);
      root.removeEventListener("pointercancel", cancelPress);
      root.removeEventListener("pointerleave", cancelPress);
      root.removeEventListener("click", onClick, true);
      root.removeEventListener("contextmenu", onContextMenu);
    };
  }, [container, selector]);
}
