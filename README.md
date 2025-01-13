# spscqueue

** Single Producer Single Consumer ring buffer queue **

[Circular buffer](https://en.wikipedia.org/wiki/Circular_buffer)  is a very simple data structure.

![](https://upload.wikimedia.org/wikipedia/commons/thumb/f/fd/Circular_Buffer_Animation.gif/400px-Circular_Buffer_Animation.gif)

The ring buffer queue provided in this repository differs from the classic data structure in the following ways:
* The length of each item in the queue is variable.
* Mainly used for inter-process communication based on shared memory
  * Provides shared memory based initialization methods (create or open)
  * Provides mechanisms such as file locks in order to ensure that there is only one producer and one consumer
* Provide cache line alignment optimizations
