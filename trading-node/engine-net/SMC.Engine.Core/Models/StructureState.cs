namespace SMC.Engine.Core.Models;

public sealed class StructureState
{
    public Bias Bias { get; internal set; } = Bias.Undetermined;
    public SwingPoint? ContinuationReference { get; internal set; }
    public SwingPoint? ReversalReference { get; internal set; }
}
