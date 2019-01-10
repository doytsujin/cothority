import BN from "bn.js";
import { randomBytes } from "crypto";
import { Point } from "../../index";
import Ed25519 from "./curve";
import Ed25519Scalar from "./scalar";

export type PointType = number | Buffer | BN

export default class Ed25519Point implements Point {
    // TODO: this should be private
    ref: {
        point: any,
        curve: Ed25519
    }

    constructor(curve: Ed25519, X?: PointType, Y?: PointType, Z?: PointType, T?: PointType) {
        if (X instanceof Buffer) {
            X = new BN(X, 16, "le");
        }
        if (Y instanceof Buffer) {
            Y = new BN(Y, 16, "le");
        }
        if (Z instanceof Buffer) {
            Z = new BN(Z, 16, "le");
        }
        if (T instanceof Buffer) {
            T = new BN(T, 16, "le");
        }
        // the point reference is stored in a Point reference to make set()
        // consistent.
        this.ref = {
            point: curve.curve.point(X, Y, Z, T),
            curve: curve
        };
    }

    string(): string {
        return this.toString()
    }

    inspect(): string {
        return this.toString()
    }

    toString(): string {
        const bytes = this.marshalBinary();
        return Array.from(bytes, b =>
        ("0" + (b & 0xff).toString(16)).slice(-2)
        ).join("");
    }

    equal(p2: Ed25519Point): boolean {
        const b1 = this.marshalBinary();
        const b2 = p2.marshalBinary();
        for (var i = 0; i < 32; i++) {
            if (b1[i] !== b2[i]) {
                return false;
            }
        }
        return true;
    }

    null(): Ed25519Point {
        this.ref.point = this.ref.curve.curve.point(0, 1, 1, 0);
        return this;
    }

    base(): Ed25519Point {
        this.ref.point = this.ref.curve.curve.point(
            this.ref.curve.curve.g.getX(),
            this.ref.curve.curve.g.getY()
        );
        return this;
    }

    pick(callback?: (length: number) => Buffer): Ed25519Point {
        return this.embed(Buffer.from([]), callback);
    }

    set(p: Ed25519Point): Ed25519Point {
        this.ref = p.ref;
        return this;
    }

    clone(): Ed25519Point {
        const { point } = this.ref;
        return new Ed25519Point(this.ref.curve, point.x, point.y, point.z, point.t);
    }

    embedLen(): number {
        // Reserve the most-significant 8 bits for pseudo-randomness.
        // Reserve the least-significant 8 bits for embedded data length.
        // (Hopefully it's unlikely we'll need >=2048-bit curves soon.)
        return Math.floor((255 - 8 - 8) / 8);
    }

    embed(data: Buffer, callback?: (length: number) => Buffer): Ed25519Point {
        let dl = this.embedLen();
        if (data.length > dl) {
            throw new Error("data.length > embedLen");
        }

        if (dl > data.length) {
            dl = data.length;
        }

        callback = callback || randomBytes;

        let point_obj = new Ed25519Point(this.ref.curve);
        while (true) {
            const buff = callback(32);

            if (dl > 0) {
                buff[0] = dl; // encode length in lower 8 bits
                data.copy(buff, 1) // copy data into buff starting from the 2nd position
            }

            try {
                point_obj.unmarshalBinary(buff);
            } catch (e) {
                continue; // try again
            }
            if (dl == 0) {
                point_obj.ref.point = point_obj.ref.point.mul(new BN(8));
                if (point_obj.ref.point.isInfinity()) {
                    continue; // unlucky
                }
                return point_obj;
            }

            let q = point_obj.clone();
            q.ref.point = q.ref.point.mul(this.ref.curve.curve.n);
            if (q.ref.point.isInfinity()) {
                return point_obj;
            }
        }
    }

    data(): Buffer {
        const bytes = this.marshalBinary();
        const dl = bytes[0];
        if (dl > this.embedLen()) {
            throw new Error("invalid embedded data length");
        }
        return bytes.slice(1, dl + 1);
    }

    add(p1: Ed25519Point, p2: Ed25519Point): Ed25519Point {
        const point = p1.ref.point;
        this.ref.point = this.ref.curve.curve
            .point(point.x, point.y, point.z, point.t)
            .add(p2.ref.point);
        return this;
    }

    sub(p1: Ed25519Point, p2: Ed25519Point): Ed25519Point {
        const point = p1.ref.point;
        this.ref.point = this.ref.curve.curve
            .point(point.x, point.y, point.z, point.t)
            .add(p2.ref.point.neg());
        return this;
    }

    neg(p: Ed25519Point): Ed25519Point {
        this.ref.point = p.ref.point.neg();
        return this;
    }

    mul(s: Ed25519Scalar, p?: Ed25519Point): Ed25519Point {
        p = p || null;
        const arr = s.ref.arr;
        this.ref.point =
        p !== null ? p.ref.point.mul(arr) : this.ref.curve.curve.g.mul(arr);
        return this;
    }

    marshalSize(): number {
        return 32;
    }

    marshalBinary(): Buffer {
        this.ref.point.normalize();

        const buffer = this.ref.point.getY().toArray("le", 32);
        buffer[31] ^= (this.ref.point.x.isOdd() ? 1 : 0) << 7;

        return Buffer.from(buffer);
    }

    unmarshalBinary(bytes: Buffer): void {
        // we create a copy because the array might be modified
        const buff = Buffer.from(bytes);

        const odd = buff[31] >> 7 === 1;

        buff[31] &= 0x7f;
        let bnp = new BN(buff, 16, "le");
        if (bnp.cmp(this.ref.curve.curve.p) >= 0) {
            throw new Error("bytes > p");
        }
        this.ref.point = this.ref.curve.curve.pointFromY(bnp, odd);
    }
}