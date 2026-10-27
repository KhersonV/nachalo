import * as THREE from "three";
import { loadHexImage } from "./resources";

/** Scene-owned textures backed by the single bounded decoded-image cache. */
export class HexTextureLoader extends THREE.TextureLoader {
  override load(url: string, onLoad?: (texture: THREE.Texture<HTMLImageElement>) => void, _onProgress?: (event: ProgressEvent) => void, onError?: (error: unknown) => void): THREE.Texture<HTMLImageElement> {
    const texture = new THREE.Texture<HTMLImageElement>();
    this.manager.itemStart(url);
    void loadHexImage(url).then(image => {
      texture.image = image;
      texture.needsUpdate = true;
      onLoad?.(texture);
    }, error => { this.manager.itemError(url); onError?.(error); }).finally(() => this.manager.itemEnd(url));
    return texture;
  }
}
